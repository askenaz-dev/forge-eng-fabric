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
			r.Get("/assets/{assetID}/versions/{version}", srv.getAssetVersion)
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
	Owners         []string       `json:"owners"`
	Metadata       map[string]any `json:"metadata"`
}

var validTypes = map[string]struct{}{
	"mcp": {}, "skill": {}, "agent": {}, "workflow": {}, "prompt_template": {},
}

var validVisibility = map[string]struct{}{"workspace": {}, "tenant": {}}

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
	q := `SELECT id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,owners,metadata,created_at,COALESCE(created_by,'')
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
		if err := rows.Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy); err != nil {
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
	if req.LifecycleState != "" && req.LifecycleState != "proposed" {
		writeJSON(w, 409, map[string]string{"code": "conflict", "message": "Phase 0 only supports lifecycle_state=proposed; full lifecycle transitions arrive in Phase 1"})
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
	var a Asset
	err = s.pool.QueryRow(r.Context(),
		`INSERT INTO asset(id,version,type,name,description,owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,owners,metadata,created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10,$11,'proposed',$12,$13::jsonb,$14)
		 RETURNING id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,owners,metadata,created_at,COALESCE(created_by,'')`,
		assetID, req.Version, req.Type, req.Name, req.Description, req.OwnerTeam, string(inputsBytes), string(outputsBytes), wsID, tenantID, req.Visibility, req.Owners, string(metaBytes), sub).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
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
		`SELECT id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,owners,metadata,created_at,COALESCE(created_by,'')
		 FROM asset WHERE id=$1 ORDER BY created_at DESC LIMIT 1`, id).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, a)
}

func (s *server) getAssetVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetID")
	v := chi.URLParam(r, "version")
	var a Asset
	err := s.pool.QueryRow(r.Context(),
		`SELECT id,version,type,name,COALESCE(description,''),owner_team,inputs_schema,outputs_schema,workspace_id,tenant_id,visibility,lifecycle_state,owners,metadata,created_at,COALESCE(created_by,'')
		 FROM asset WHERE id=$1 AND version=$2`, id, v).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.OwnerTeam, &a.InputsSchema, &a.OutputsSchema, &a.WorkspaceID, &a.TenantID, &a.Visibility, &a.LifecycleState, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, a)
}
