package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/twmb/franz-go/pkg/kgo"
)

// gatewayPublishRequest is the payload accepted by the gateway-publish
// lifecycle hook. Skill/agent payloads supply (digest, signature, attestation);
// MCPs declare a remote_transport contract instead.
type gatewayPublishRequest struct {
	Channel         string                 `json:"channel"`
	PackageDigest   string                 `json:"package_digest"`
	SignatureID     string                 `json:"signature_id"`
	AttestationID   string                 `json:"attestation_id"`
	BytesURI        string                 `json:"bytes_uri"`
	SizeBytes       int64                  `json:"size_bytes"`
	RemoteTransport map[string]any         `json:"remote_transport,omitempty"`
}

var validGatewayChannels = map[string]struct{}{"stable": {}, "beta": {}}

// gatewayPublishHook implements the `lifecycle-hooks/gateway-publish` endpoint.
// Preconditions: asset is approved, T1+, and (for skill/agent) carries a signed
// package; for MCP, carries a non-empty remote_transport contract. The hook
// atomically writes asset_package (when applicable), sets the asset's
// distribution_* columns and emits com.forge.asset.gateway_published.v1.
func (s *server) gatewayPublishHook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetID")
	v := chi.URLParam(r, "version")
	var req gatewayPublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	if req.Channel == "" {
		req.Channel = "stable"
	}
	if _, ok := validGatewayChannels[req.Channel]; !ok {
		writeJSON(w, 400, map[string]string{"code": "invalid_channel", "message": "channel must be stable or beta"})
		return
	}

	// Eligibility: approved + T1+ minimum.
	var assetType, lifecycle, trustLevel string
	err := s.pool.QueryRow(r.Context(),
		`SELECT type, lifecycle_state, COALESCE(trust_level,'T0') FROM asset WHERE id=$1 AND version=$2`, id, v).
		Scan(&assetType, &lifecycle, &trustLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "not found", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if lifecycle != "approved" {
		writeJSON(w, 409, map[string]string{"code": "distribution_invariant_violated", "message": "asset must be approved before gateway publication"})
		return
	}
	if trustLevel == "T0" {
		writeJSON(w, 409, map[string]string{"code": "distribution_invariant_violated", "message": "trust_level must be T1 or higher to publish to the gateway"})
		return
	}

	// Per-type preconditions.
	switch assetType {
	case "skill", "agent":
		if req.PackageDigest == "" || req.SignatureID == "" || req.AttestationID == "" || req.BytesURI == "" {
			writeJSON(w, 400, map[string]string{"code": "signature_invalid", "message": "package_digest, signature_id, attestation_id and bytes_uri are required for skill/agent"})
			return
		}
	case "mcp":
		if len(req.RemoteTransport) == 0 {
			writeJSON(w, 409, map[string]string{"code": "remote_transport_required", "message": "MCP assets must declare a remote_transport contract to be gateway-publishable"})
			return
		}
	default:
		writeJSON(w, 400, map[string]string{"code": "not_packageable", "message": "only skill, agent and mcp assets can be gateway-published"})
		return
	}

	// Authorization: same FGA scope as edit on the asset's workspace.
	sub, _ := r.Context().Value(subjectKey).(string)
	var workspaceID uuid.UUID
	if err := s.pool.QueryRow(r.Context(), `SELECT workspace_id FROM asset WHERE id=$1 AND version=$2`, id, v).Scan(&workspaceID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	ok, err := s.fga.Check(r.Context(), "user:"+sub, "can_edit", "workspace:"+workspaceID.String())
	if err != nil {
		http.Error(w, "fga check failed: "+err.Error(), 500)
		return
	}
	if !ok {
		http.Error(w, "forbidden", 403)
		return
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	now := time.Now().UTC()
	switch assetType {
	case "skill", "agent":
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO asset_package(asset_id,version,digest,signature_id,attestation_id,bytes_uri,size_bytes,channel)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			id, v, req.PackageDigest, req.SignatureID, req.AttestationID, req.BytesURI, req.SizeBytes, req.Channel,
		); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeJSON(w, 409, map[string]string{"code": "package_already_published", "message": "this (asset_id,version) is already packaged; bump version to rotate"})
				return
			}
			http.Error(w, err.Error(), 500)
			return
		}
	}

	var a Asset
	digestArg := any(nil)
	if req.PackageDigest != "" {
		digestArg = req.PackageDigest
	}
	signedAtArg := any(now)
	if req.PackageDigest == "" {
		signedAtArg = nil
	}
	if err := rowScanAsset(tx.QueryRow(r.Context(),
		`UPDATE asset
		   SET distribution_gateway_published = true,
		       distribution_gateway_channel   = $3,
		       distribution_package_digest    = COALESCE($4, distribution_package_digest),
		       distribution_package_signed_at = COALESCE($5, distribution_package_signed_at)
		 WHERE id=$1 AND version=$2
		 RETURNING `+assetSelectColumns,
		id, v, req.Channel, digestArg, signedAtArg), &a); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	s.publishAssetGatewayPublishedEvent(r.Context(), a, "com.forge.asset.gateway_published.v1", req)
	writeJSON(w, 200, a)
}

// autoUnpublishOnLifecycle flips distribution_gateway_published to false when an
// asset moves to deprecated/retired, emits the corresponding unpublished event,
// and returns the refreshed Asset row. Returns (nil, nil) if the asset was not
// gateway-published in the first place.
func (s *server) autoUnpublishOnLifecycle(r *http.Request, id, version string) (*Asset, error) {
	var wasPublished bool
	err := s.pool.QueryRow(r.Context(),
		`SELECT COALESCE(distribution_gateway_published,false) FROM asset WHERE id=$1 AND version=$2`, id, version).
		Scan(&wasPublished)
	if err != nil {
		return nil, err
	}
	if !wasPublished {
		return nil, nil
	}
	var a Asset
	if err := rowScanAsset(s.pool.QueryRow(r.Context(),
		`UPDATE asset SET distribution_gateway_published=false WHERE id=$1 AND version=$2 RETURNING `+assetSelectColumns,
		id, version), &a); err != nil {
		return nil, err
	}
	s.publishAssetGatewayPublishedEvent(r.Context(), a, "com.forge.asset.gateway_unpublished.v1", gatewayPublishRequest{Channel: a.Distribution.GatewayChannel})
	return &a, nil
}

// publishAssetGatewayPublishedEvent emits the gateway_published / gateway_unpublished
// CloudEvents to the same bus used by every other registry event.
func (s *server) publishAssetGatewayPublishedEvent(ctx context.Context, a Asset, eventType string, req gatewayPublishRequest) {
	cid, _ := ctx.Value(cidKey).(string)
	sub, _ := ctx.Value(subjectKey).(string)
	data := map[string]any{
		"asset_id":     a.ID,
		"version":      a.Version,
		"tenant_id":    a.TenantID,
		"workspace_id": a.WorkspaceID,
		"type":         a.Type,
		"channel":      a.Distribution.GatewayChannel,
	}
	if a.Distribution.PackageDigest != nil {
		data["package_digest"] = *a.Distribution.PackageDigest
	}
	if len(req.RemoteTransport) > 0 {
		data["remote_transport"] = req.RemoteTransport
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
		"data":               data,
	}
	body, _ := json.Marshal(envelope)
	_ = s.kc.ProduceSync(ctx, &kgo.Record{
		Topic: s.topic,
		Key:   []byte(a.TenantID.String()),
		Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte(eventType)},
			{Key: "content-type", Value: []byte("application/cloudevents+json")},
		},
	}).FirstErr()
}
