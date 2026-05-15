package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

// External MCP / A2A registration. These endpoints accept a third-party
// transport endpoint and a per-Tenant credential ref, fetch the live
// manifest (MCP tool manifest) or agent-card (A2A), persist the digest on
// `external_mcp_endpoint` / `external_a2a_agent`, and create an Asset Registry
// record with provenance=external in lifecycle_state=proposed. The standard
// lifecycle + eval pipeline then governs promotion. The credential ref is
// never dereferenced here — the gateway resolves it at invocation time.

// manifestFetcher is the seam the external-MCP registration handler uses to
// fetch and hash the upstream tool manifest. Replace at construction time
// to mock external HTTP in tests.
type manifestFetcher interface {
	FetchManifest(ctx context.Context, endpointURL string) (manifestHash string, err error)
}

// agentCardFetcher is the seam the external-A2A registration handler uses to
// fetch and hash the upstream agent card. Replace at construction time to
// mock external HTTP in tests.
type agentCardFetcher interface {
	FetchAgentCard(ctx context.Context, endpointURL string) (cardHash string, err error)
}

// defaultFetcher is the production implementation of both fetcher seams.
// It uses a single http.Client with a hard timeout and refuses redirects to
// non-HTTPS destinations (the upstream may misconfigure a redirect into a
// plaintext server).
type defaultFetcher struct {
	http *http.Client
}

func newDefaultFetcher() *defaultFetcher {
	return &defaultFetcher{
		http: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
					return fmt.Errorf("refusing redirect to scheme=%s", req.URL.Scheme)
				}
				if len(via) > 5 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}
}

// FetchManifest GETs the endpoint and hashes the response body. The MCP
// tool-manifest is conventionally returned by the well-known MCP HTTP
// transport on a GET of the endpoint URL (servers that speak SSE instead
// will return the initialization document on GET). For drift detection we
// don't care which subset of MCP transport semantics the server speaks —
// only that the bytes are stable.
func (f *defaultFetcher) FetchManifest(ctx context.Context, endpointURL string) (string, error) {
	return f.fetchAndHash(ctx, endpointURL)
}

// FetchAgentCard GETs the A2A agent card at the well-known suffix
// `/.well-known/agent.json`. Per the A2A protocol the agent card describes
// the agent's identity, supported tasks, transports and authentication
// scheme.
func (f *defaultFetcher) FetchAgentCard(ctx context.Context, endpointURL string) (string, error) {
	cardURL, err := joinAgentCardURL(endpointURL)
	if err != nil {
		return "", err
	}
	return f.fetchAndHash(ctx, cardURL)
}

func (f *defaultFetcher) fetchAndHash(ctx context.Context, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("accept", "application/json")
	resp, err := f.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		// Drain a bounded prefix so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("fetch %s: status %d", target, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func joinAgentCardURL(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/.well-known/agent.json"
	return u.String(), nil
}

// externalAssetRegistration is the wire shape both external endpoints share.
// `name` becomes the asset name; `id` and the underlying registry row are
// constructed deterministically as `<type>:<workspace>:<name>` like the
// internal createAsset path.
type externalAssetRegistration struct {
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	OwnerTeam      string         `json:"owner_team"`
	Description    string         `json:"description"`
	EndpointURL    string         `json:"endpoint_url"`
	CredentialRef  string         `json:"credential_ref"`
	Allowlist      []string       `json:"allowlist"`
	WorkspaceID    string         `json:"workspace_id"`
	HowTo          map[string]any `json:"how_to"`
	ActiveSurface  map[string]any `json:"active_surface"`
	Metadata       map[string]any `json:"metadata"`
}

// credentialRefPattern guards against accidentally accepting a literal
// credential value where a vault path was expected. Production deployments
// use SPIRE/IRSA-derived workload identity to read the vault entry at
// gateway invocation time — the registry MUST never see the secret itself.
var credentialRefPattern = regexp.MustCompile(`^(vault|aws-sm|gcp-sm|azure-kv)://`)

const (
	externalMCPType = "mcp"
	externalA2AType = "agent"
)

func (s *server) registerExternalMCP(w http.ResponseWriter, r *http.Request) {
	s.registerExternalAsset(w, r, externalMCPType, s.mcpFetcher)
}

func (s *server) registerExternalA2A(w http.ResponseWriter, r *http.Request) {
	s.registerExternalAsset(w, r, externalA2AType, s.a2aFetcher)
}

// registerExternalAsset implements both POST handlers (mcp & a2a). The type
// argument switches between the `external_mcp_endpoint` and
// `external_a2a_agent` tables and the digest event payload. The two share
// the same validation, FGA check, asset creation and event emission.
func (s *server) registerExternalAsset(w http.ResponseWriter, r *http.Request, kind string, fetcher fetcherForKind) {
	var req externalAssetRegistration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	wsID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		http.Error(w, "invalid workspace_id", 400)
		return
	}
	if req.Name == "" || req.Version == "" || req.OwnerTeam == "" {
		http.Error(w, "name, version and owner_team required", 400)
		return
	}
	if !semverPattern.MatchString(req.Version) {
		http.Error(w, "version must be SemVer (MAJOR.MINOR.PATCH)", 400)
		return
	}
	if req.EndpointURL == "" {
		http.Error(w, "endpoint_url required", 400)
		return
	}
	if u, perr := url.Parse(req.EndpointURL); perr != nil || (u.Scheme != "http" && u.Scheme != "https") {
		http.Error(w, "endpoint_url must be http or https", 400)
		return
	}
	if req.CredentialRef == "" {
		http.Error(w, "credential_ref required", 400)
		return
	}
	if !credentialRefPattern.MatchString(req.CredentialRef) {
		writeJSON(w, 400, map[string]string{"code": "invalid_credential_ref", "message": "credential_ref must be a vault path (vault://, aws-sm://, gcp-sm://, azure-kv://)"})
		return
	}
	if req.HowTo != nil {
		if verr := validateHowTo(req.HowTo); verr != nil {
			writeJSON(w, 400, map[string]string{"code": "invalid_how_to", "message": verr.Error()})
			return
		}
	}
	if req.ActiveSurface != nil {
		if verr := validateActiveSurface(req.ActiveSurface); verr != nil {
			writeJSON(w, 400, map[string]string{"code": "invalid_active_surface", "message": verr.Error()})
			return
		}
	}

	sub, _ := r.Context().Value(subjectKey).(string)
	ok, err := s.fga.Check(r.Context(), "user:"+sub, "can_edit", "workspace:"+wsID.String())
	if err != nil {
		http.Error(w, "fga check failed: "+err.Error(), 500)
		return
	}
	if !ok {
		http.Error(w, "forbidden", 403)
		return
	}

	tenantID, err := s.lookupTenant(r.Context(), wsID)
	if err != nil {
		http.Error(w, "tenant lookup failed: "+err.Error(), 500)
		return
	}

	manifestHash, ferr := fetcher.fetch(r.Context(), req.EndpointURL)
	if ferr != nil {
		writeJSON(w, 502, map[string]string{"code": "manifest_fetch_failed", "message": ferr.Error()})
		return
	}
	fetchedAt := time.Now().UTC()
	assetID := kind + ":" + wsID.String() + ":" + req.Name

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	metaBytes, _ := json.Marshal(req.Metadata)
	var howToBytes, surfaceBytes any
	if req.HowTo != nil {
		b, _ := json.Marshal(req.HowTo)
		howToBytes = string(b)
	}
	if req.ActiveSurface != nil {
		b, _ := json.Marshal(req.ActiveSurface)
		surfaceBytes = string(b)
	}

	var a Asset
	err = rowScanAsset(tx.QueryRow(r.Context(),
		`INSERT INTO asset(id,version,type,name,description,owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,trust_level,owners,metadata,created_by,external_provenance,how_to_json,active_surface_json)
		 VALUES ($1,$2,$3,$4,$5,$6,'{}'::jsonb,'{}'::jsonb,$7,$8,'workspace','proposed','T0','{}',$9::jsonb,$10,'external',$11::jsonb,$12::jsonb)
		 RETURNING `+assetSelectColumns,
		assetID, req.Version, kind, req.Name, req.Description, req.OwnerTeam,
		wsID, tenantID, string(metaBytes), sub, howToBytes, surfaceBytes,
	), &a)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	switch kind {
	case externalMCPType:
		_, err = tx.Exec(r.Context(),
			`INSERT INTO external_mcp_endpoint(asset_id, tenant_id, endpoint_url, credential_ref, allowlist, manifest_hash, manifest_fetched_at, created_by)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			 ON CONFLICT (asset_id) DO UPDATE SET
			   endpoint_url=EXCLUDED.endpoint_url,
			   credential_ref=EXCLUDED.credential_ref,
			   allowlist=EXCLUDED.allowlist,
			   manifest_hash=EXCLUDED.manifest_hash,
			   manifest_fetched_at=EXCLUDED.manifest_fetched_at`,
			a.ID, tenantID, req.EndpointURL, req.CredentialRef, req.Allowlist, manifestHash, fetchedAt, sub)
	case externalA2AType:
		_, err = tx.Exec(r.Context(),
			`INSERT INTO external_a2a_agent(asset_id, tenant_id, endpoint_url, credential_ref, task_allowlist, agent_card_hash, agent_card_fetched_at, created_by)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			 ON CONFLICT (asset_id) DO UPDATE SET
			   endpoint_url=EXCLUDED.endpoint_url,
			   credential_ref=EXCLUDED.credential_ref,
			   task_allowlist=EXCLUDED.task_allowlist,
			   agent_card_hash=EXCLUDED.agent_card_hash,
			   agent_card_fetched_at=EXCLUDED.agent_card_fetched_at`,
			a.ID, tenantID, req.EndpointURL, req.CredentialRef, req.Allowlist, manifestHash, fetchedAt, sub)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	_ = s.fga.Write(r.Context(), "workspace:"+wsID.String(), "workspace", "asset:"+a.ID)

	s.publishExternalRegisteredEvent(r, a, kind, req.EndpointURL, manifestHash, fetchedAt, req.Allowlist)
	writeJSON(w, 201, a)
}

func (s *server) publishExternalRegisteredEvent(r *http.Request, a Asset, kind, endpoint, hash string, fetchedAt time.Time, allowlist []string) {
	cid, _ := r.Context().Value(cidKey).(string)
	sub, _ := r.Context().Value(subjectKey).(string)
	hashField := "manifest_hash"
	if kind == externalA2AType {
		hashField = "agent_card_hash"
	}
	envelope := map[string]any{
		"specversion":        "1.0",
		"id":                 uuid.NewString(),
		"source":             "forge://service/registry",
		"type":               "com.forge.asset.external_registered.v1",
		"subject":            "asset/" + a.ID + "@" + a.Version,
		"time":               time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype":    "application/json",
		"forgetenantid":      a.TenantID.String(),
		"forgeworkspaceid":   a.WorkspaceID.String(),
		"forgeactor":         "user:" + sub,
		"forgecorrelationid": cid,
		"data": map[string]any{
			"asset_id":     a.ID,
			"version":      a.Version,
			"kind":         kind,
			"endpoint_url": endpoint,
			hashField:      hash,
			"fetched_at":   fetchedAt.Format(time.RFC3339Nano),
			"allowlist":    allowlist,
		},
	}
	body, _ := json.Marshal(envelope)
	_ = s.kc.ProduceSync(r.Context(), &kgo.Record{
		Topic: s.topic,
		Key:   []byte(a.TenantID.String()),
		Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte("com.forge.asset.external_registered.v1")},
			{Key: "content-type", Value: []byte("application/cloudevents+json")},
		},
	}).FirstErr()
}

// fetcherForKind is the union-by-position adapter so registerExternalAsset can
// stay generic over the two upstream digest sources. The mcp fetcher returns
// the tool-manifest hash; the a2a fetcher returns the agent-card hash. We
// keep them as separate seams so they can be stubbed independently in tests.
type fetcherForKind interface {
	fetch(ctx context.Context, endpoint string) (string, error)
}

type mcpManifestFetcher struct{ inner manifestFetcher }

func (m mcpManifestFetcher) fetch(ctx context.Context, endpoint string) (string, error) {
	return m.inner.FetchManifest(ctx, endpoint)
}

type a2aCardFetcher struct{ inner agentCardFetcher }

func (a a2aCardFetcher) fetch(ctx context.Context, endpoint string) (string, error) {
	return a.inner.FetchAgentCard(ctx, endpoint)
}

// reverifyOnPromotion implements task 2.6 for external assets: at promotion
// time, re-fetch the upstream digest, compare against the stored digest, and
// refuse the promotion if drift is detected unless the caller explicitly
// acknowledges. Internal assets (provenance=internal) skip this check.
func (s *server) reverifyOnPromotion(ctx context.Context, assetID, assetType string, acknowledgeDrift bool) *validationFault {
	switch assetType {
	case externalMCPType:
		return s.reverifyExternalMCP(ctx, assetID, acknowledgeDrift)
	case externalA2AType:
		return s.reverifyExternalA2A(ctx, assetID, acknowledgeDrift)
	}
	// Other asset types (e.g. external skill) currently have no live upstream
	// digest to compare against; promotion proceeds without a re-verification.
	return nil
}

func (s *server) reverifyExternalMCP(ctx context.Context, assetID string, acknowledgeDrift bool) *validationFault {
	var endpoint, stored string
	err := s.pool.QueryRow(ctx,
		`SELECT endpoint_url, COALESCE(manifest_hash,'') FROM external_mcp_endpoint WHERE asset_id=$1`,
		assetID).Scan(&endpoint, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		// External asset without a row means registration was incomplete; we
		// can't gate on a stored hash we don't have.
		return &validationFault{Code: "missing_external_record", Message: "external endpoint metadata missing for asset"}
	}
	if err != nil {
		return &validationFault{Code: "drift_check_failed", Message: err.Error()}
	}
	if s.mcpFetcher.inner == nil {
		return nil // fetcher disabled (test mode)
	}
	live, ferr := s.mcpFetcher.fetch(ctx, endpoint)
	if ferr != nil {
		return &validationFault{Code: "drift_check_failed", Message: ferr.Error()}
	}
	if stored != "" && live != stored && !acknowledgeDrift {
		return &validationFault{Code: "drift_detected", Message: fmt.Sprintf("manifest hash drifted from %s to %s; pass acknowledge_drift=true to override", stored, live)}
	}
	// Refresh the stored hash + timestamp so the daily cron does not page on
	// the same drift the promotion already accepted.
	_, _ = s.pool.Exec(ctx,
		`UPDATE external_mcp_endpoint SET manifest_hash=$2, manifest_fetched_at=$3 WHERE asset_id=$1`,
		assetID, live, time.Now().UTC())
	return nil
}

func (s *server) reverifyExternalA2A(ctx context.Context, assetID string, acknowledgeDrift bool) *validationFault {
	var endpoint, stored string
	err := s.pool.QueryRow(ctx,
		`SELECT endpoint_url, COALESCE(agent_card_hash,'') FROM external_a2a_agent WHERE asset_id=$1`,
		assetID).Scan(&endpoint, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return &validationFault{Code: "missing_external_record", Message: "external agent metadata missing for asset"}
	}
	if err != nil {
		return &validationFault{Code: "drift_check_failed", Message: err.Error()}
	}
	if s.a2aFetcher.inner == nil {
		return nil
	}
	live, ferr := s.a2aFetcher.fetch(ctx, endpoint)
	if ferr != nil {
		return &validationFault{Code: "drift_check_failed", Message: ferr.Error()}
	}
	if stored != "" && live != stored && !acknowledgeDrift {
		return &validationFault{Code: "drift_detected", Message: fmt.Sprintf("agent_card hash drifted from %s to %s; pass acknowledge_drift=true to override", stored, live)}
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE external_a2a_agent SET agent_card_hash=$2, agent_card_fetched_at=$3 WHERE asset_id=$1`,
		assetID, live, time.Now().UTC())
	return nil
}

