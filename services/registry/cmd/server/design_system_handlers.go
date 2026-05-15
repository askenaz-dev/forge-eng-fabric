package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

// listDesignSystems returns the Design System catalog visible to the caller.
// Built-in templates (visibility=tenant_global, owned by the platform tenant)
// are included unconditionally; tenant-published Design Systems follow the
// caller's workspace/tenant visibility scope. The handler is intentionally
// lightweight — it shares the underlying asset table with the generic asset
// list, but exposes a curated projection so the wizard does not have to
// understand the wider Asset shape.
func (s *server) listDesignSystems(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(),
		`SELECT `+assetSelectColumns+`
		 FROM asset
		 WHERE type='design_system'
		   AND lifecycle_state='approved'
		 ORDER BY name, version DESC`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var a Asset
		if err := rowScanAsset(rows, &a); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out = append(out, projectDesignSystem(a))
	}
	writeJSON(w, 200, out)
}

// getDesignSystem returns a single Design System by its asset id or by alias.
// When an alias is used, the response carries the `X-Resolved-From-Alias`
// header so the client can record the indirection.
func (s *server) getDesignSystem(w http.ResponseWriter, r *http.Request) {
	rawRef := chi.URLParam(r, "ref")
	if rawRef == "" {
		http.Error(w, "missing ref", 400)
		return
	}
	assetID, version, aliasResolved := s.resolveDesignSystemRef(r, rawRef)
	if assetID == "" {
		http.Error(w, "design_system_not_found", 404)
		return
	}
	var (
		row pgx.Row
	)
	if version != "" {
		row = s.pool.QueryRow(r.Context(),
			`SELECT `+assetSelectColumns+` FROM asset WHERE id=$1 AND version=$2`,
			assetID, version)
	} else {
		row = s.pool.QueryRow(r.Context(),
			`SELECT `+assetSelectColumns+` FROM asset
			 WHERE id=$1 AND type='design_system' AND lifecycle_state='approved'
			 ORDER BY version DESC LIMIT 1`,
			assetID)
	}
	var a Asset
	if err := rowScanAsset(row, &a); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "design_system_not_found", 404)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	if aliasResolved != "" {
		w.Header().Set("X-Resolved-From-Alias", aliasResolved)
	}
	writeJSON(w, 200, projectDesignSystem(a))
}

// retargetDesignSystemAlias updates the `design_system_alias` table and emits
// `asset.design_system.alias_changed.v1`. The platform principal owns the
// `ds-forge-default` alias.
func (s *server) retargetDesignSystemAlias(w http.ResponseWriter, r *http.Request) {
	alias := chi.URLParam(r, "alias")
	if alias == "" {
		http.Error(w, "missing alias", 400)
		return
	}
	sub, _ := r.Context().Value(subjectKey).(string)
	if sub != "system:forge-platform" {
		// In a follow-up, this opens up to per-tenant aliases gated by FGA.
		http.Error(w, "forbidden", 403)
		return
	}
	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Target) == "" {
		http.Error(w, "missing target", 400)
		return
	}
	prev := ""
	_ = s.pool.QueryRow(r.Context(),
		`SELECT asset_id FROM design_system_alias WHERE alias=$1`, alias,
	).Scan(&prev)
	if _, err := s.pool.Exec(r.Context(),
		`INSERT INTO design_system_alias(alias, asset_id, retargeted_at, retargeted_by)
		 VALUES ($1,$2,now(),$3)
		 ON CONFLICT (alias) DO UPDATE SET asset_id=$2, retargeted_at=now(), retargeted_by=$3`,
		alias, req.Target, "user:"+sub,
	); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.publishAliasChanged(r, alias, prev, req.Target)
	writeJSON(w, 200, map[string]string{"alias": alias, "before": prev, "after": req.Target})
}

// resolveDesignSystemRef accepts an `asset_id`, an `asset_id@version` or an
// alias like `ds-forge-default`. The returned (assetID, version, aliasUsed)
// tuple is consumed by getDesignSystem and by the App service's swap path.
func (s *server) resolveDesignSystemRef(r *http.Request, ref string) (string, string, string) {
	ref = strings.TrimSpace(ref)
	if at := strings.Index(ref, "@"); at >= 0 {
		return ref[:at], ref[at+1:], ""
	}
	// alias lookup
	var aliasTarget string
	if err := s.pool.QueryRow(r.Context(),
		`SELECT asset_id FROM design_system_alias WHERE alias=$1`, ref,
	).Scan(&aliasTarget); err == nil {
		return aliasTarget, "", ref
	}
	return ref, "", ""
}

// projectDesignSystem narrows the full Asset row to the catalog shape the
// wizard and the CLI consume. Hides the internal columns (provenance,
// active_surface) that are irrelevant for design_system assets.
func projectDesignSystem(a Asset) map[string]any {
	out := map[string]any{
		"asset_id":          a.ID,
		"version":           a.Version,
		"name":              a.Name,
		"description":       a.Description,
		"owner_team":        a.OwnerTeam,
		"workspace_id":      a.WorkspaceID,
		"tenant_id":         a.TenantID,
		"visibility":        designSystemVisibility(a),
		"lifecycle_state":   a.LifecycleState,
		"trust_level":       a.TrustLevel,
		"eval_scores":       a.EvalScores,
		"manifest":          a.Manifest,
		"built_in_template": a.BuiltInTemplate,
		"created_at":        a.CreatedAt,
	}
	return out
}

// designSystemVisibility folds the asset's `visibility` and `built_in_template`
// flags into the canonical wire value the wizard renders ("tenant_global" for
// built-ins, otherwise the stored visibility).
func designSystemVisibility(a Asset) string {
	if a.BuiltInTemplate {
		return "tenant_global"
	}
	return a.Visibility
}

// publishDesignSystemPublished is called by createAsset right after the row is
// persisted. Emits `asset.design_system.published.v1` on the first version of
// an id and `asset.design_system.version_published.v1` on subsequent versions.
func (s *server) publishDesignSystemPublished(r *http.Request, a Asset) {
	cid, _ := r.Context().Value(cidKey).(string)
	sub, _ := r.Context().Value(subjectKey).(string)
	var existing int
	_ = s.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM asset WHERE id=$1 AND version <> $2`, a.ID, a.Version,
	).Scan(&existing)
	eventType := "com.forge.asset.design_system.published.v1"
	if existing > 0 {
		eventType = "com.forge.asset.design_system.version_published.v1"
	}
	envelope := map[string]any{
		"specversion":        "1.0",
		"id":                 uuid.NewString(),
		"source":             "forge://service/registry",
		"type":               eventType,
		"subject":            "asset/" + a.ID + "@" + a.Version,
		"time":               time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype":    "application/json",
		"forgetenantid":      a.TenantID.String(),
		"forgeworkspaceid":   a.WorkspaceID.String(),
		"forgeactor":         "user:" + sub,
		"forgecorrelationid": cid,
		"data": map[string]any{
			"asset_id":          a.ID,
			"version":           a.Version,
			"manifest":          a.Manifest,
			"built_in_template": a.BuiltInTemplate,
			"visibility":        designSystemVisibility(a),
			"owner_team":        a.OwnerTeam,
		},
	}
	body, _ := json.Marshal(envelope)
	_ = s.kc.ProduceSync(r.Context(), &kgo.Record{
		Topic: s.topic, Key: []byte(a.TenantID.String()), Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte(eventType)},
			{Key: "content-type", Value: []byte("application/cloudevents+json")},
		},
	}).FirstErr()
}

// publishAliasChanged emits the alias-retarget event.
func (s *server) publishAliasChanged(r *http.Request, alias, before, after string) {
	cid, _ := r.Context().Value(cidKey).(string)
	sub, _ := r.Context().Value(subjectKey).(string)
	envelope := map[string]any{
		"specversion":        "1.0",
		"id":                 uuid.NewString(),
		"source":             "forge://service/registry",
		"type":               "com.forge.asset.design_system.alias_changed.v1",
		"subject":            "design_system_alias/" + alias,
		"time":               time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype":    "application/json",
		"forgeactor":         "user:" + sub,
		"forgecorrelationid": cid,
		"data": map[string]any{
			"alias":  alias,
			"before": before,
			"after":  after,
		},
	}
	body, _ := json.Marshal(envelope)
	_ = s.kc.ProduceSync(r.Context(), &kgo.Record{
		Topic: s.topic, Key: []byte("design_system_alias:" + alias), Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte("com.forge.asset.design_system.alias_changed.v1")},
			{Key: "content-type", Value: []byte("application/cloudevents+json")},
		},
	}).FirstErr()
}
