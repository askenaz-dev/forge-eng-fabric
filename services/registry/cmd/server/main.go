// Asset registry — Phase 0 minimal CRUD with lifecycle=proposed only.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	cfg := loadConfig()

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

	srv := &server{pool: pool, keys: keys, kc: kc, topic: cfg.EventsTopic, issuer: cfg.KeycloakIssuer, audience: cfg.KeycloakAudience}

	r.Group(func(r chi.Router) {
		r.Use(srv.requireJWT)
		r.Route("/v1", func(r chi.Router) {
			r.Get("/workspaces/{workspaceID}/assets", srv.listAssets)
			r.Post("/workspaces/{workspaceID}/assets", srv.createAsset)
			r.Get("/assets/{assetID}", srv.getAssetLatest)
			r.Get("/assets/{assetID}/versions/{version}", srv.getAssetVersion)
		})
	})

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: r, ReadHeaderTimeout: 10 * time.Second}
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

// --- handlers ---------------------------------------------------------

type Asset struct {
	ID             string         `json:"id"`
	Version        string         `json:"version"`
	Type           string         `json:"type"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	WorkspaceID    uuid.UUID      `json:"workspace_id"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	LifecycleState string         `json:"lifecycle_state"`
	Owners         []string       `json:"owners"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	CreatedBy      string         `json:"created_by"`
}

type assetCreate struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Owners      []string       `json:"owners"`
	Metadata    map[string]any `json:"metadata"`
}

var validTypes = map[string]struct{}{
	"skill": {}, "prompt": {}, "mcp": {}, "workflow": {}, "application": {},
	"repo_template": {}, "eval_dataset": {}, "healing_action": {},
}

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
	typeFilter := r.URL.Query().Get("type")
	q := `SELECT id,version,type,name,COALESCE(description,''),workspace_id,tenant_id,lifecycle_state,owners,metadata,created_at,COALESCE(created_by,'')
		  FROM asset WHERE workspace_id=$1`
	args := []any{wsID}
	if typeFilter != "" {
		q += " AND type=$2"
		args = append(args, typeFilter)
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
		if err := rows.Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.WorkspaceID, &a.TenantID, &a.LifecycleState, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy); err != nil {
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
	sub, _ := r.Context().Value(subjectKey).(string)

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
	var a Asset
	err = s.pool.QueryRow(r.Context(),
		`INSERT INTO asset(id,version,type,name,description,workspace_id,tenant_id,lifecycle_state,owners,metadata,created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'proposed',$8,$9::jsonb,$10)
		 RETURNING id,version,type,name,COALESCE(description,''),workspace_id,tenant_id,lifecycle_state,owners,metadata,created_at,COALESCE(created_by,'')`,
		assetID, req.Version, req.Type, req.Name, req.Description, wsID, tenantID, req.Owners, string(metaBytes), sub).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.WorkspaceID, &a.TenantID, &a.LifecycleState, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
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
		`SELECT id,version,type,name,COALESCE(description,''),workspace_id,tenant_id,lifecycle_state,owners,metadata,created_at,COALESCE(created_by,'')
		 FROM asset WHERE id=$1 ORDER BY created_at DESC LIMIT 1`, id).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.WorkspaceID, &a.TenantID, &a.LifecycleState, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
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
		`SELECT id,version,type,name,COALESCE(description,''),workspace_id,tenant_id,lifecycle_state,owners,metadata,created_at,COALESCE(created_by,'')
		 FROM asset WHERE id=$1 AND version=$2`, id, v).
		Scan(&a.ID, &a.Version, &a.Type, &a.Name, &a.Description, &a.WorkspaceID, &a.TenantID, &a.LifecycleState, &a.Owners, &a.Metadata, &a.CreatedAt, &a.CreatedBy)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, a)
}
