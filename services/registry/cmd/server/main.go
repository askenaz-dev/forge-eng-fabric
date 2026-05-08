// Asset registry — Phase 0 minimal CRUD with lifecycle=proposed only.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/forge-eng-fabric/services/registry/internal/telemetry"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg := loadConfig()
	shutdownTelemetry, err := telemetry.Init(context.Background(), "registry", cfg.Environment, cfg.OTLPEndpoint)
	if err != nil {
		log.Fatalf("otel: %v", err)
	}
	defer func() { _ = shutdownTelemetry(context.Background()) }()

	pool, err := pgxpool.New(context.Background(), cfg.PostgresURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	jwksURL := strings.TrimRight(cfg.KeycloakIssuer, "/") + "/protocol/openid-connect/certs"
	keys, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		log.Fatalf("jwks: %v", err)
	}

	kc, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(cfg.KafkaBrokers, ",")...), kgo.AllowAutoTopicCreation())
	if err != nil {
		log.Fatalf("kafka: %v", err)
	}
	defer kc.Close()

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(correlationID)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db not ready", 503)
			return
		}
		w.WriteHeader(200)
	})

	srv := &server{
		pool:     pool,
		keys:     keys,
		kc:       kc,
		fga:      newOpenFGAClient(cfg.OpenFGAURL, cfg.OpenFGAStoreID, cfg.OpenFGAModelID),
		topic:    cfg.EventsTopic,
		issuer:   cfg.KeycloakIssuer,
		audience: cfg.KeycloakAudience,
	}

	r.Group(func(r chi.Router) {
		r.Use(srv.requireJWT)
		r.Route("/v1", func(r chi.Router) {
			r.Get("/workspaces/{workspaceID}/assets", srv.listAssets)
			r.Post("/workspaces/{workspaceID}/assets", srv.createAsset)
			r.Get("/assets/{assetID}", srv.getAssetLatest)
			r.Get("/assets/{assetID}/deployments", srv.listAssetDeployments)
			r.Post("/assets/{assetID}/deployments", srv.recordAssetDeployment)
			r.Get("/assets/{assetID}/versions/{version}", srv.getAssetVersion)
			r.Post("/assets/{assetID}/versions/{version}/transition", srv.transitionAsset)
			r.Post("/assets/{assetID}/versions/{version}/lifecycle-hooks/pipeline-green", srv.pipelineGreenHook)
			r.Post("/assets/{assetID}/versions/{version}/lifecycle-hooks/workspace-owner-approval", srv.workspaceOwnerApprovalHook)
			r.Post("/assets/{assetID}/versions/{version}/invoke-check", srv.checkAssetInvocation)
		})
	})

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: otelhttp.NewHandler(r, "registry"), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("registry listening on %s", cfg.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

type config struct {
	Addr             string
	PostgresURL      string
	KeycloakIssuer   string
	KeycloakAudience string
	KafkaBrokers     string
	EventsTopic      string
	OpenFGAURL       string
	OpenFGAStoreID   string
	OpenFGAModelID   string
	Environment      string
	OTLPEndpoint     string
}

func loadConfig() config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	return config{
		Addr:             get("ADDR", ":8082"),
		PostgresURL:      get("POSTGRES_URL", "postgres://forge:forge@localhost:5432/forge_registry?sslmode=disable"),
		KeycloakIssuer:   get("KEYCLOAK_ISSUER", "http://localhost:8080/realms/forge"),
		KeycloakAudience: get("KEYCLOAK_AUDIENCE", "forge-control-plane"),
		KafkaBrokers:     get("KAFKA_BROKERS", "localhost:29092"),
		EventsTopic:      get("EVENTS_TOPIC", "forge.events"),
		OpenFGAURL:       get("OPENFGA_API_URL", "http://localhost:8088"),
		OpenFGAStoreID:   get("OPENFGA_STORE_ID", ""),
		OpenFGAModelID:   get("OPENFGA_AUTHORIZATION_MODEL_ID", ""),
		Environment:      get("ENV", "local"),
		OTLPEndpoint:     get("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
	}
}

// --- middleware -------------------------------------------------------

type ctxKey int

const (
	cidKey ctxKey = iota
	subjectKey
)

func correlationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Correlation-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cidKey, id)))
	})
}

type server struct {
	pool     *pgxpool.Pool
	keys     keyfunc.Keyfunc
	kc       *kgo.Client
	fga      *openFGAClient
	topic    string
	issuer   string
	audience string
}

func (s *server) requireJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			http.Error(w, "missing bearer token", 401)
			return
		}
		tok, err := jwt.Parse(strings.TrimSpace(h[7:]), s.keys.Keyfunc, jwt.WithValidMethods([]string{"RS256"}))
		if err != nil || !tok.Valid {
			http.Error(w, "invalid token", 401)
			return
		}
		claims := tok.Claims.(jwt.MapClaims)
		if iss, _ := claims["iss"].(string); iss != s.issuer {
			http.Error(w, "bad issuer", 401)
			return
		}
		if !audienceMatches(claims["aud"], s.audience) {
			http.Error(w, "bad audience", 401)
			return
		}
		sub, _ := claims["sub"].(string)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), subjectKey, sub)))
	})
}

func audienceMatches(aud any, want string) bool {
	switch v := aud.(type) {
	case string:
		return v == want
	case []any:
		for _, x := range v {
			if s, _ := x.(string); s == want {
				return true
			}
		}
	}
	return false
}

type openFGAClient struct {
	url     string
	storeID string
	modelID string
	http    *http.Client
}

func newOpenFGAClient(url, storeID, modelID string) *openFGAClient {
	return &openFGAClient{url: strings.TrimRight(url, "/"), storeID: storeID, modelID: modelID, http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *openFGAClient) Check(ctx context.Context, user, relation, object string) (bool, error) {
	if c.storeID == "" {
		return true, nil
	}
	body, _ := json.Marshal(map[string]any{
		"authorization_model_id": c.modelID,
		"tuple_key": map[string]string{
			"user":     user,
			"relation": relation,
			"object":   object,
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/stores/%s/check", c.url, c.storeID), bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return false, fmt.Errorf("openfga check %d: %s", resp.StatusCode, string(data))
	}
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return false, err
	}
	return out.Allowed, nil
}

func (c *openFGAClient) Write(ctx context.Context, user, relation, object string) error {
	if c.storeID == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]any{
		"authorization_model_id": c.modelID,
		"writes": map[string]any{
			"tuple_keys": []map[string]string{{"user": user, "relation": relation, "object": object}},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/stores/%s/write", c.url, c.storeID), bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openfga write %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

// --- handlers ---------------------------------------------------------

type Asset struct {
	ID             string         `json:"id"`
	Version        string         `json:"version"`
	Type           string         `json:"type"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	OwnerTeam      string         `json:"owner_team"`
	InputsSchema   map[string]any `json:"inputs_schema"`
	OutputsSchema  map[string]any `json:"outputs_schema"`
	WorkspaceID    uuid.UUID      `json:"workspace_id"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	Visibility     string         `json:"visibility"`
	LifecycleState string         `json:"lifecycle_state"`
	TrustLevel     string         `json:"trust_level"`
	EvalScores     map[string]any `json:"eval_scores"`
	Owners         []string       `json:"owners"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	CreatedBy      string         `json:"created_by"`
}

type assetCreate struct {
	Type           string         `json:"type"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Version        string         `json:"version"`
	OwnerTeam      string         `json:"owner_team"`
	InputsSchema   map[string]any `json:"inputs_schema"`
	OutputsSchema  map[string]any `json:"outputs_schema"`
	Visibility     string         `json:"visibility"`
	LifecycleState string         `json:"lifecycle_state"`
	TrustLevel     string         `json:"trust_level"`
	EvalScores     map[string]any `json:"eval_scores"`
	Owners         []string       `json:"owners"`
	Metadata       map[string]any `json:"metadata"`
}

type AssetDeployment struct {
	ID                  string    `json:"id"`
	AssetID             string    `json:"asset_id"`
	Env                 string    `json:"env"`
	RevisionID          string    `json:"revision_id"`
	ImageDigest         string    `json:"image_digest"`
	RuntimeID           string    `json:"runtime_id"`
	VerifiedStatus      string    `json:"verified_status"`
	SignatureVerified   bool      `json:"signature_verified"`
	AttestationVerified bool      `json:"attestation_verified"`
	OpenSpecIDs         []string  `json:"openspec_ids"`
	PRSHA               string    `json:"pr_sha,omitempty"`
	Actor               string    `json:"actor,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type recordAssetDeploymentRequest struct {
	ID                  string   `json:"id"`
	Env                 string   `json:"env"`
	RevisionID          string   `json:"revision_id"`
	ImageDigest         string   `json:"image_digest"`
	RuntimeID           string   `json:"runtime_id"`
	VerifiedStatus      string   `json:"verified_status"`
	SignatureVerified   bool     `json:"signature_verified"`
	AttestationVerified bool     `json:"attestation_verified"`
	OpenSpecIDs         []string `json:"openspec_ids"`
	PRSHA               string   `json:"pr_sha"`
	Actor               string   `json:"actor"`
}

var validTypes = map[string]struct{}{
	"mcp": {}, "skill": {}, "agent": {}, "workflow": {}, "prompt_template": {},
	"application": {}, "repo_template": {}, "eval_dataset": {},
	"healing_action": {},
}

var validVisibility = map[string]struct{}{"workspace": {}, "tenant": {}}

var validLifecycle = map[string]struct{}{"proposed": {}, "in_review": {}, "approved": {}, "deprecated": {}, "retired": {}}

var validTrustLevels = map[string]struct{}{"T0": {}, "T1": {}, "T2": {}, "T3": {}, "T4": {}, "T5": {}}

var evalThresholds = map[string]float64{"T0": 0, "T1": 0.8, "T2": 0.85, "T3": 0.9, "T4": 0.95, "T5": 0.98}

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) listAssets(w http.ResponseWriter, r *http.Request) {
	wsID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		http.Error(w, "invalid workspaceID", 400)
		return
	}
	sub, _ := r.Context().Value(subjectKey).(string)
	ok, err := s.fga.Check(r.Context(), "user:"+sub, "can_view", "workspace:"+wsID.String())
	if err != nil {
		http.Error(w, "fga check failed: "+err.Error(), 500)
		return
	}
	if !ok {
		http.Error(w, "forbidden", 403)
		return
	}
	q := `SELECT id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,COALESCE(trust_level,'T0'),COALESCE(eval_scores,'{}'::jsonb),owners,metadata,created_at,COALESCE(created_by,'')
		  FROM asset WHERE workspace_id=$1`
	args := []any{wsID}
	for _, filter := range []struct{ key, column string }{
		{"type", "type"},
		{"owner_team", "owner_team"},
		{"visibility", "visibility"},
		{"lifecycle_state", "lifecycle_state"},
	} {
		if value := r.URL.Query().Get(filter.key); value != "" {
			q += fmt.Sprintf(" AND %s=$%d", filter.column, len(args)+1)
			args = append(args, value)
		}
	}
	q += " ORDER BY name, version"
	rows, err := s.pool.Query(r.Context(), q, args...)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	out := []Asset{}
	for rows.Next() {
		var a Asset
		if err := rows.Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.TrustLevel, &a.EvalScores, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out = append(out, a)
	}
	writeJSON(w, 200, out)
}

func (s *server) createAsset(w http.ResponseWriter, r *http.Request) {
	wsID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		http.Error(w, "invalid workspaceID", 400)
		return
	}
	var req assetCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	if _, ok := validTypes[req.Type]; !ok {
		http.Error(w, "invalid type", 400)
		return
	}
	if req.Name == "" || req.Version == "" {
		http.Error(w, "name and version required", 400)
		return
	}
	if req.OwnerTeam == "" || req.InputsSchema == nil || req.OutputsSchema == nil {
		http.Error(w, "owner_team, inputs_schema and outputs_schema are required", 400)
		return
	}
	if req.Visibility == "" {
		req.Visibility = "workspace"
	}
	if _, ok := validVisibility[req.Visibility]; !ok {
		http.Error(w, "invalid visibility", 400)
		return
	}
	if req.LifecycleState == "" {
		req.LifecycleState = "proposed"
	}
	if _, ok := validLifecycle[req.LifecycleState]; !ok {
		http.Error(w, "invalid lifecycle_state", 400)
		return
	}
	if req.TrustLevel == "" {
		req.TrustLevel = "T0"
	}
	if _, ok := validTrustLevels[req.TrustLevel]; !ok {
		http.Error(w, "invalid trust_level", 400)
		return
	}
	if !semverPattern.MatchString(req.Version) {
		http.Error(w, "version must be SemVer (MAJOR.MINOR.PATCH)", 400)
		return
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

	// Resolve tenant_id from workspace via cross-DB query? In Phase 0 the registry
	// keeps its own copy of tenant_id supplied by the caller via the workspace
	// referenced. For simplicity we look it up in our own copy of the workspace
	// table; if not present we ask the control plane DB through a shared DB user.
	// To keep this self-contained, we accept a tenant_id derived from a side-call:
	// here we just look it up in the control_plane DB schema if it exists; fallback
	// to a deterministic synthetic uuid keyed by workspace.
	tenantID, err := s.lookupTenant(r.Context(), wsID)
	if err != nil {
		http.Error(w, "tenant lookup failed: "+err.Error(), 500)
		return
	}

	// id = "<type>:<workspace>:<name>" — stable so repeated POSTs with
	// new versions chain correctly; (id,version) UNIQUE prevents re-writes.
	assetID := req.Type + ":" + wsID.String() + ":" + req.Name

	metaBytes, _ := json.Marshal(req.Metadata)
	inputsBytes, _ := json.Marshal(req.InputsSchema)
	outputsBytes, _ := json.Marshal(req.OutputsSchema)
	evalBytes, _ := json.Marshal(normalizeEvalScores(req.EvalScores))
	var a Asset
	err = s.pool.QueryRow(r.Context(),
		`INSERT INTO asset(id,version,type,name,description,owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,trust_level,eval_scores,owners,metadata,created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10,$11,$12,$13,$14::jsonb,$15,$16::jsonb,$17)
		 RETURNING id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,COALESCE(trust_level,'T0'),COALESCE(eval_scores,'{}'::jsonb),owners,metadata,created_at,COALESCE(created_by,'')`,
		assetID, req.Version, req.Type, req.Name, req.Description, req.OwnerTeam, string(inputsBytes), string(outputsBytes), wsID, tenantID, req.Visibility, req.LifecycleState, req.TrustLevel, string(evalBytes), req.Owners, string(metaBytes), sub).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.TrustLevel, &a.EvalScores, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, 409, map[string]string{"code": "conflict", "message": "asset version already exists; bump version"})
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	_ = s.fga.Write(r.Context(), "workspace:"+wsID.String(), "workspace", "asset:"+a.ID)
	for _, owner := range req.Owners {
		_ = s.fga.Write(r.Context(), "user:"+owner, "owner", "asset:"+a.ID)
	}

	// Emit asset.created.v1
	cid, _ := r.Context().Value(cidKey).(string)
	envelope := map[string]any{
		"specversion":        "1.0",
		"id":                 uuid.NewString(),
		"source":             "forge://service/registry",
		"type":               "com.forge.asset.created.v1",
		"subject":            "asset/" + a.ID + "@" + a.Version,
		"time":               time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype":    "application/json",
		"forgetenantid":      a.TenantID.String(),
		"forgeworkspaceid":   a.WorkspaceID.String(),
		"forgeactor":         "user:" + sub,
		"forgecorrelationid": cid,
		"data": map[string]any{
			"asset_id":        a.ID,
			"version":         a.Version,
			"tenant_id":       a.TenantID,
			"workspace_id":    a.WorkspaceID,
			"type":            a.Type,
			"name":            a.Name,
			"owner_team":      a.OwnerTeam,
			"visibility":      a.Visibility,
			"inputs_schema":   a.InputsSchema,
			"outputs_schema":  a.OutputsSchema,
			"lifecycle_state": "proposed",
			"owners":          a.Owners,
			"metadata":        a.Metadata,
			"created_at":      a.CreatedAt,
			"created_by":      sub,
		},
	}
	body, _ := json.Marshal(envelope)
	_ = s.kc.ProduceSync(r.Context(), &kgo.Record{
		Topic: s.topic, Key: []byte(a.TenantID.String()), Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte("com.forge.asset.created.v1")},
			{Key: "content-type", Value: []byte("application/cloudevents+json")},
		},
	}).FirstErr()

	writeJSON(w, 201, a)
}

func (s *server) lookupTenant(ctx context.Context, wsID uuid.UUID) (uuid.UUID, error) {
	// Reuse the same Postgres server, switching DB via dblink would be heavy.
	// In docker-compose the control-plane DB is reachable; we open a tiny
	// connection here. For Phase 0 scope we just open a per-call conn.
	cpURL := os.Getenv("CONTROL_PLANE_DB_URL")
	if cpURL == "" {
		cpURL = "postgres://forge:forge@localhost:5432/forge_control_plane?sslmode=disable"
	}
	conn, err := pgx.Connect(ctx, cpURL)
	if err != nil {
		return uuid.Nil, err
	}
	defer conn.Close(ctx)
	var tid uuid.UUID
	err = conn.QueryRow(ctx, `SELECT tenant_id FROM workspace WHERE id=$1`, wsID).Scan(&tid)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, errors.New("workspace not found")
	}
	return tid, err
}

func (s *server) getAssetLatest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetID")
	var a Asset
	err := s.pool.QueryRow(r.Context(),
		`SELECT id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,COALESCE(trust_level,'T0'),COALESCE(eval_scores,'{}'::jsonb),owners,metadata,created_at,COALESCE(created_by,'')
		 FROM asset WHERE id=$1 ORDER BY created_at DESC LIMIT 1`, id).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.TrustLevel, &a.EvalScores, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, a)
}

func (s *server) recordAssetDeployment(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "assetID")
	var req recordAssetDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	if req.VerifiedStatus == "" {
		req.VerifiedStatus = "unknown"
	}
	openSpecBytes, _ := json.Marshal(req.OpenSpecIDs)
	var d AssetDeployment
	err := s.pool.QueryRow(r.Context(),
		`INSERT INTO asset_deployment(id, asset_id, env, revision_id, image_digest, runtime_id, verified_status, signature_verified, attestation_verified, openspec_ids, pr_sha, actor)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12)
		 RETURNING id, asset_id, env, revision_id, image_digest, runtime_id, verified_status, signature_verified, attestation_verified, openspec_ids, COALESCE(pr_sha,''), COALESCE(actor,''), created_at`,
		req.ID, assetID, req.Env, req.RevisionID, req.ImageDigest, req.RuntimeID, req.VerifiedStatus, req.SignatureVerified, req.AttestationVerified, string(openSpecBytes), req.PRSHA, req.Actor).
		Scan(&d.ID, &d.AssetID, &d.Env, &d.RevisionID, &d.ImageDigest, &d.RuntimeID, &d.VerifiedStatus, &d.SignatureVerified, &d.AttestationVerified, &d.OpenSpecIDs, &d.PRSHA, &d.Actor, &d.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, 409, map[string]string{"code": "conflict", "message": "deployment revision already recorded"})
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 201, d)
}

func (s *server) listAssetDeployments(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "assetID")
	env := r.URL.Query().Get("env")
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		_, _ = fmt.Sscanf(raw, "%d", &limit)
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	cursor := r.URL.Query().Get("cursor")
	q := `SELECT id, asset_id, env, revision_id, image_digest, runtime_id, verified_status, signature_verified, attestation_verified, openspec_ids, COALESCE(pr_sha,''), COALESCE(actor,''), created_at
	      FROM asset_deployment WHERE asset_id=$1`
	args := []any{assetID}
	if env != "" {
		q += fmt.Sprintf(" AND env=$%d", len(args)+1)
		args = append(args, env)
	}
	if cursor != "" {
		q += fmt.Sprintf(" AND created_at < (SELECT created_at FROM asset_deployment WHERE id=$%d)", len(args)+1)
		args = append(args, cursor)
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args)+1)
	args = append(args, limit+1)
	rows, err := s.pool.Query(r.Context(), q, args...)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	out := []AssetDeployment{}
	for rows.Next() {
		var d AssetDeployment
		if err := rows.Scan(&d.ID, &d.AssetID, &d.Env, &d.RevisionID, &d.ImageDigest, &d.RuntimeID, &d.VerifiedStatus, &d.SignatureVerified, &d.AttestationVerified, &d.OpenSpecIDs, &d.PRSHA, &d.Actor, &d.CreatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out = append(out, d)
	}
	next := ""
	if len(out) > limit {
		next = out[limit].ID
		out = out[:limit]
	}
	writeJSON(w, 200, map[string]any{"deployments": out, "next_cursor": next})
}

func (s *server) getAssetVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetID")
	v := chi.URLParam(r, "version")
	var a Asset
	err := s.pool.QueryRow(r.Context(),
		`SELECT id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,COALESCE(trust_level,'T0'),COALESCE(eval_scores,'{}'::jsonb),owners,metadata,created_at,COALESCE(created_by,'')
		 FROM asset WHERE id=$1 AND version=$2`, id, v).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.TrustLevel, &a.EvalScores, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, a)
}

type lifecycleTransition struct {
	LifecycleState     string         `json:"lifecycle_state"`
	TrustLevel         string         `json:"trust_level"`
	EvalScores         map[string]any `json:"eval_scores"`
	ApprovedBySDLCteam bool           `json:"approved_by_sdlc_team"`
}

func (s *server) transitionAsset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetID")
	v := chi.URLParam(r, "version")
	var req lifecycleTransition
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	if _, ok := validLifecycle[req.LifecycleState]; !ok {
		http.Error(w, "invalid lifecycle_state", 400)
		return
	}
	if req.TrustLevel == "" {
		req.TrustLevel = "T0"
	}
	if _, ok := validTrustLevels[req.TrustLevel]; !ok {
		http.Error(w, "invalid trust_level", 400)
		return
	}
	var current string
	if err := s.pool.QueryRow(r.Context(), `SELECT lifecycle_state FROM asset WHERE id=$1 AND version=$2`, id, v).Scan(&current); err != nil {
		http.Error(w, "not found", 404)
		return
	}
	if !canTransition(current, req.LifecycleState) {
		writeJSON(w, 409, map[string]string{"code": "invalid_transition", "message": "lifecycle transition is not allowed"})
		return
	}
	if req.LifecycleState == "approved" {
		if req.TrustLevel == "T5" && !req.ApprovedBySDLCteam {
			writeJSON(w, 202, map[string]any{"decision": "requires_approval", "required_approvers": []string{"sdlc-team"}})
			return
		}
		if failing := failingEvalScores(req.TrustLevel, req.EvalScores); len(failing) > 0 {
			writeJSON(w, 400, map[string]any{"code": "eval_threshold_failed", "failing": failing})
			return
		}
	}
	evalBytes, _ := json.Marshal(req.EvalScores)
	var a Asset
	err := s.pool.QueryRow(r.Context(),
		`UPDATE asset SET lifecycle_state=$3, trust_level=$4, eval_scores=$5::jsonb WHERE id=$1 AND version=$2
		 RETURNING id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,COALESCE(trust_level,'T0'),COALESCE(eval_scores,'{}'::jsonb),owners,metadata,created_at,COALESCE(created_by,'')`,
		id, v, req.LifecycleState, req.TrustLevel, string(evalBytes)).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.TrustLevel, &a.EvalScores, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.publishAssetLifecycleEvent(r, a, current, req.LifecycleState)
	writeJSON(w, 200, a)
}

type pipelineGreenHookRequest struct {
	PipelineRunID       string         `json:"pipeline_run_id"`
	CommitSHA           string         `json:"commit_sha"`
	ImageDigest         string         `json:"image_digest"`
	ImageSigned         bool           `json:"image_signed"`
	SignatureVerified   bool           `json:"signature_verified"`
	AttestationVerified bool           `json:"attestation_verified"`
	SBOMPublished       bool           `json:"sbom_published"`
	GateResults         []hookGate     `json:"gate_results"`
	TrustLevel          string         `json:"trust_level"`
	EvalScores          map[string]any `json:"eval_scores"`
}

type hookGate struct {
	Stage     string `json:"stage"`
	Outcome   string `json:"outcome"`
	ReportURL string `json:"report_url"`
}

func (s *server) pipelineGreenHook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetID")
	v := chi.URLParam(r, "version")
	var req pipelineGreenHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	if ok, reason := pipelineHookReady(req); !ok {
		writeJSON(w, 202, map[string]any{"decision": "waiting", "reason": reason})
		return
	}
	if req.TrustLevel == "" {
		req.TrustLevel = "T1"
	}
	patch := map[string]any{
		"phase2_pipeline": map[string]any{
			"pipeline_run_id":         req.PipelineRunID,
			"commit_sha":              req.CommitSHA,
			"image_digest":            req.ImageDigest,
			"image_signed":            req.ImageSigned,
			"signature_verified":      req.SignatureVerified,
			"attestation_verified":    req.AttestationVerified,
			"sbom_published":          req.SBOMPublished,
			"first_green_pipeline_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	a, from, err := s.transitionAssetWithPatch(r, id, v, "in_review", req.TrustLevel, req.EvalScores, patch)
	if err != nil {
		http.Error(w, err.Error(), statusForLifecycleError(err))
		return
	}
	s.publishAssetLifecycleEvent(r, a, from, "in_review")
	writeJSON(w, 200, a)
}

type workspaceOwnerApprovalHookRequest struct {
	ApprovalID string         `json:"approval_id"`
	ApprovedBy string         `json:"approved_by"`
	Comment    string         `json:"comment"`
	TrustLevel string         `json:"trust_level"`
	EvalScores map[string]any `json:"eval_scores"`
}

func (s *server) workspaceOwnerApprovalHook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetID")
	v := chi.URLParam(r, "version")
	var req workspaceOwnerApprovalHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	if req.ApprovalID == "" || req.ApprovedBy == "" {
		writeJSON(w, 202, map[string]any{"decision": "requires_approval", "required_approvers": []string{"workspace-owner"}})
		return
	}
	if req.TrustLevel == "" {
		req.TrustLevel = "T3"
	}
	if failing := failingEvalScores(req.TrustLevel, req.EvalScores); len(failing) > 0 {
		writeJSON(w, 400, map[string]any{"code": "eval_threshold_failed", "failing": failing})
		return
	}
	patch := map[string]any{
		"phase2_approval": map[string]any{
			"approval_id": req.ApprovalID,
			"approved_by": req.ApprovedBy,
			"approved_at": time.Now().UTC().Format(time.RFC3339Nano),
			"comment":     req.Comment,
		},
	}
	a, from, err := s.transitionAssetWithPatch(r, id, v, "approved", req.TrustLevel, req.EvalScores, patch)
	if err != nil {
		http.Error(w, err.Error(), statusForLifecycleError(err))
		return
	}
	s.publishAssetLifecycleEvent(r, a, from, "approved")
	writeJSON(w, 200, a)
}

func pipelineHookReady(req pipelineGreenHookRequest) (bool, string) {
	if !req.ImageSigned || !req.SignatureVerified || !req.AttestationVerified {
		return false, "image signature and attestation must be verified"
	}
	if !req.SBOMPublished {
		return false, "sbom must be published"
	}
	if len(req.GateResults) == 0 {
		return false, "pipeline gates are missing"
	}
	for _, gate := range req.GateResults {
		if gate.Outcome != "pass" && gate.Outcome != "warn" {
			return false, "pipeline gate " + gate.Stage + " is not green"
		}
	}
	return true, "ready"
}

func (s *server) transitionAssetWithPatch(r *http.Request, id, version, nextState, trustLevel string, evalScores map[string]any, metadataPatch map[string]any) (Asset, string, error) {
	if _, ok := validLifecycle[nextState]; !ok {
		return Asset{}, "", fmt.Errorf("invalid lifecycle_state")
	}
	if trustLevel == "" {
		trustLevel = "T0"
	}
	if _, ok := validTrustLevels[trustLevel]; !ok {
		return Asset{}, "", fmt.Errorf("invalid trust_level")
	}
	var current string
	if err := s.pool.QueryRow(r.Context(), `SELECT lifecycle_state FROM asset WHERE id=$1 AND version=$2`, id, version).Scan(&current); err != nil {
		return Asset{}, "", fmt.Errorf("not found")
	}
	if !canTransition(current, nextState) {
		return Asset{}, current, fmt.Errorf("invalid_transition")
	}
	evalBytes, _ := json.Marshal(normalizeEvalScores(evalScores))
	patchBytes, _ := json.Marshal(metadataPatch)
	var a Asset
	err := s.pool.QueryRow(r.Context(),
		`UPDATE asset SET lifecycle_state=$3, trust_level=$4, eval_scores=$5::jsonb, metadata=metadata || $6::jsonb WHERE id=$1 AND version=$2
		 RETURNING id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,COALESCE(trust_level,'T0'),COALESCE(eval_scores,'{}'::jsonb),owners,metadata,created_at,COALESCE(created_by,'')`,
		id, version, nextState, trustLevel, string(evalBytes), string(patchBytes)).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.TrustLevel, &a.EvalScores, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		return Asset{}, current, err
	}
	return a, current, nil
}

func statusForLifecycleError(err error) int {
	if strings.Contains(err.Error(), "not found") {
		return http.StatusNotFound
	}
	if strings.Contains(err.Error(), "invalid_transition") {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func normalizeEvalScores(scores map[string]any) map[string]any {
	if scores == nil {
		return map[string]any{}
	}
	return scores
}

type invokeCheck struct {
	Environment string `json:"environment"`
}

func (s *server) checkAssetInvocation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetID")
	v := chi.URLParam(r, "version")
	var req invokeCheck
	_ = json.NewDecoder(r.Body).Decode(&req)
	var state string
	var workspaceID uuid.UUID
	var tenantID uuid.UUID
	var trustLevel string
	err := s.pool.QueryRow(
		r.Context(),
		`SELECT lifecycle_state, workspace_id, tenant_id, COALESCE(trust_level,'T0') FROM asset WHERE id=$1 AND version=$2`,
		id,
		v,
	).Scan(&state, &workspaceID, &tenantID, &trustLevel)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	allowed, reason := invocationAllowed(state, req.Environment)
	eventType := "com.forge.asset.invocation.checked.v1"
	s.publishAssetInvocationCheckedEvent(r, id, v, workspaceID, tenantID, state, trustLevel, req.Environment, allowed, reason, eventType)
	writeJSON(w, 200, map[string]any{
		"allowed":          allowed,
		"reason":           reason,
		"audit_event_type": eventType,
		"correlation_id":   r.Context().Value(cidKey),
	})
}

func canTransition(from, to string) bool {
	allowed := map[string][]string{
		"proposed":   {"in_review", "retired"},
		"in_review":  {"approved", "proposed", "retired"},
		"approved":   {"deprecated", "retired"},
		"deprecated": {"retired"},
		"retired":    {},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return from == to
}

func failingEvalScores(trustLevel string, scores map[string]any) map[string]any {
	threshold := evalThresholds[trustLevel]
	failing := map[string]any{}
	for _, key := range []string{"quality", "safety", "cost", "latency"} {
		value, ok := numeric(scores[key])
		if !ok || value < threshold {
			failing[key] = scores[key]
		}
	}
	return failing
}

func numeric(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func invocationAllowed(lifecycleState, environment string) (bool, string) {
	if environment == "prod" || environment == "production" || environment == "staging" {
		if lifecycleState != "approved" {
			return false, "production-relevant flows require approved assets"
		}
	}
	return true, "allowed"
}

func (s *server) publishAssetLifecycleEvent(r *http.Request, a Asset, from, to string) {
	cid, _ := r.Context().Value(cidKey).(string)
	sub, _ := r.Context().Value(subjectKey).(string)
	evalBytes, _ := json.Marshal(a.EvalScores)
	_, _ = s.pool.Exec(r.Context(),
		`INSERT INTO asset_lifecycle_event(asset_id, version, from_state, to_state, trust_level, eval_scores, actor)
		 VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)`,
		a.ID, a.Version, from, to, a.TrustLevel, string(evalBytes), "user:"+sub)
	envelope := map[string]any{
		"specversion":        "1.0",
		"id":                 uuid.NewString(),
		"source":             "forge://service/registry",
		"type":               "com.forge.asset.lifecycle.transitioned.v1",
		"subject":            "asset/" + a.ID + "@" + a.Version,
		"time":               time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype":    "application/json",
		"forgetenantid":      a.TenantID.String(),
		"forgeworkspaceid":   a.WorkspaceID.String(),
		"forgeactor":         "user:" + sub,
		"forgecorrelationid": cid,
		"data":               map[string]any{"asset_id": a.ID, "version": a.Version, "from": from, "to": to, "trust_level": a.TrustLevel, "eval_scores": a.EvalScores},
	}
	body, _ := json.Marshal(envelope)
	_ = s.kc.ProduceSync(r.Context(), &kgo.Record{Topic: s.topic, Key: []byte(a.TenantID.String()), Value: body}).FirstErr()
}

func (s *server) publishAssetInvocationCheckedEvent(
	r *http.Request,
	assetID string,
	version string,
	workspaceID uuid.UUID,
	tenantID uuid.UUID,
	lifecycleState string,
	trustLevel string,
	environment string,
	allowed bool,
	reason string,
	eventType string,
) {
	cid, _ := r.Context().Value(cidKey).(string)
	sub, _ := r.Context().Value(subjectKey).(string)
	envelope := buildAssetInvocationCheckedEvent(
		assetID,
		version,
		workspaceID,
		tenantID,
		lifecycleState,
		trustLevel,
		environment,
		allowed,
		reason,
		eventType,
		cid,
		sub,
	)
	body, _ := json.Marshal(envelope)
	_ = s.kc.ProduceSync(r.Context(), &kgo.Record{
		Topic: s.topic,
		Key:   []byte(tenantID.String()),
		Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte(eventType)},
			{Key: "ce_correlation_id", Value: []byte(cid)},
		},
	}).FirstErr()
}

func buildAssetInvocationCheckedEvent(
	assetID string,
	version string,
	workspaceID uuid.UUID,
	tenantID uuid.UUID,
	lifecycleState string,
	trustLevel string,
	environment string,
	allowed bool,
	reason string,
	eventType string,
	correlationID string,
	subject string,
) map[string]any {
	return map[string]any{
		"specversion":        "1.0",
		"id":                 uuid.NewString(),
		"source":             "forge://service/registry",
		"type":               eventType,
		"subject":            "asset/" + assetID + "@" + version,
		"time":               time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype":    "application/json",
		"forgetenantid":      tenantID.String(),
		"forgeworkspaceid":   workspaceID.String(),
		"forgeactor":         "user:" + subject,
		"forgecorrelationid": correlationID,
		"data": map[string]any{
			"asset_id":        assetID,
			"version":         version,
			"lifecycle_state": lifecycleState,
			"trust_level":     trustLevel,
			"environment":     environment,
			"allowed":         allowed,
			"reason":          reason,
		},
	}
}
