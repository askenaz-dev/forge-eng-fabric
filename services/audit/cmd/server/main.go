// Audit service: consumes forge.events, persists to audit_event with
// per-tenant prev_hash chain (DB trigger), and exposes a query API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/forge-eng-fabric/services/audit/internal/telemetry"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg := loadConfig()
	shutdownTelemetry, err := telemetry.Init(context.Background(), "audit-service", cfg.Environment, cfg.OTLPEndpoint)
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

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(cfg.KafkaBrokers, ",")...),
		kgo.ConsumerGroup("audit"),
		kgo.ConsumeTopics(cfg.EventsTopic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		log.Fatalf("kafka: %v", err)
	}
	defer consumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go consumeLoop(ctx, consumer, pool)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db not ready", 503)
			return
		}
		w.WriteHeader(200)
	})

	srv := &queryServer{pool: pool, keys: keys, issuer: cfg.KeycloakIssuer, audience: cfg.KeycloakAudience}
	r.Group(func(r chi.Router) {
		r.Use(srv.requireJWT)
		r.Get("/v1/audit", srv.listAudit)
		r.Get("/v1/audit/verify", srv.verifyAuditChain)
	})

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: otelhttp.NewHandler(r, "audit-service"), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("audit listening on %s", cfg.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	cancel()
	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	_ = httpSrv.Shutdown(shCtx)
}

type config struct {
	Addr             string
	PostgresURL      string
	KeycloakIssuer   string
	KeycloakAudience string
	KafkaBrokers     string
	EventsTopic      string
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
		Addr:             get("ADDR", ":8083"),
		PostgresURL:      get("POSTGRES_URL", "postgres://forge:forge@localhost:15432/forge_audit?sslmode=disable"),
		KeycloakIssuer:   get("KEYCLOAK_ISSUER", "http://localhost:8080/realms/forge"),
		KeycloakAudience: get("KEYCLOAK_AUDIENCE", "forge-control-plane"),
		KafkaBrokers:     get("KAFKA_BROKERS", "localhost:29092"),
		EventsTopic:      get("EVENTS_TOPIC", "forge.events"),
		Environment:      get("ENV", "local"),
		OTLPEndpoint:     get("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
	}
}

// --- consumer ---------------------------------------------------------

type cloudEvent struct {
	ID                 string          `json:"id"`
	Source             string          `json:"source"`
	Type               string          `json:"type"`
	Subject            string          `json:"subject"`
	Time               time.Time       `json:"time"`
	ForgeTenantID      string          `json:"forgetenantid"`
	ForgeWorkspaceID   string          `json:"forgeworkspaceid"`
	ForgeActor         string          `json:"forgeactor"`
	ForgeCorrelationID string          `json:"forgecorrelationid"`
	Data               json.RawMessage `json:"data"`
}

func consumeLoop(ctx context.Context, c *kgo.Client, pool *pgxpool.Pool) {
	for {
		fetches := c.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				log.Printf("kafka fetch err: %v", e.Err)
			}
			continue
		}
		fetches.EachRecord(func(rec *kgo.Record) {
			var ev cloudEvent
			if err := json.Unmarshal(rec.Value, &ev); err != nil {
				log.Printf("bad event: %v", err)
				return
			}
			if ev.ForgeTenantID == "" {
				return
			}
			tenant, err := uuid.Parse(ev.ForgeTenantID)
			if err != nil {
				return
			}
			var ws *uuid.UUID
			if ev.ForgeWorkspaceID != "" {
				if u, err := uuid.Parse(ev.ForgeWorkspaceID); err == nil {
					ws = &u
				}
			}
			if _, err := pool.Exec(ctx,
				`INSERT INTO audit_event(tenant_id,workspace_id,actor,action,resource,outcome,details,correlation_id,prev_hash,hash,occurred_at)
				 VALUES ($1,$2,$3,$4,$5,'success',$6::jsonb,$7,'','',$8)`,
				tenant, ws, ev.ForgeActor, ev.Type, ev.Subject, string(ev.Data), ev.ForgeCorrelationID, ev.Time); err != nil {
				log.Printf("insert audit: %v", err)
				return
			}
		})
		if err := c.CommitUncommittedOffsets(ctx); err != nil {
			log.Printf("commit: %v", err)
		}
	}
}

// --- query API --------------------------------------------------------

type queryServer struct {
	pool     *pgxpool.Pool
	keys     keyfunc.Keyfunc
	issuer   string
	audience string
}

type AuditRow struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	WorkspaceID   *uuid.UUID `json:"workspace_id,omitempty"`
	Actor         string     `json:"actor"`
	Action        string     `json:"action"`
	Resource      string     `json:"resource"`
	Outcome       string     `json:"outcome"`
	Details       any        `json:"details"`
	CorrelationID string     `json:"correlation_id"`
	PrevHash      string     `json:"prev_hash"`
	Hash          string     `json:"hash"`
	OccurredAt    time.Time  `json:"occurred_at"`
}

type VerifyResult struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	CheckedRows  int       `json:"checked_rows"`
	InvalidRows  int       `json:"invalid_rows"`
	ChainIsValid bool      `json:"chain_is_valid"`
}

func (s *queryServer) requireJWT(next http.Handler) http.Handler {
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
		next.ServeHTTP(w, r)
	})
}

func (s *queryServer) listAudit(w http.ResponseWriter, r *http.Request) {
	tenantStr := r.URL.Query().Get("tenant_id")
	if tenantStr == "" {
		http.Error(w, "tenant_id is required", 400)
		return
	}
	tid, err := uuid.Parse(tenantStr)
	if err != nil {
		http.Error(w, "invalid tenant_id", 400)
		return
	}
	// Optional filters
	q := `SELECT id,tenant_id,workspace_id,actor,action,resource,outcome,details,COALESCE(correlation_id,''),prev_hash,hash,occurred_at
         FROM audit_event WHERE tenant_id=$1`
	args := []any{tid}
	if ws := r.URL.Query().Get("workspace_id"); ws != "" {
		wid, err := uuid.Parse(ws)
		if err != nil {
			http.Error(w, "invalid workspace_id", 400)
			return
		}
		q += fmt.Sprintf(" AND workspace_id = $%d", len(args)+1)
		args = append(args, wid)
	}
	if actor := r.URL.Query().Get("actor"); actor != "" {
		q += fmt.Sprintf(" AND actor = $%d", len(args)+1)
		args = append(args, actor)
	}
	if action := r.URL.Query().Get("action"); action != "" {
		q += fmt.Sprintf(" AND action = $%d", len(args)+1)
		args = append(args, action)
	}
	// Time window filters (ISO8601)
	if since := r.URL.Query().Get("since"); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			http.Error(w, "invalid since", 400)
			return
		}
		q += fmt.Sprintf(" AND occurred_at >= $%d", len(args)+1)
		args = append(args, t)
	}
	if until := r.URL.Query().Get("until"); until != "" {
		t, err := time.Parse(time.RFC3339, until)
		if err != nil {
			http.Error(w, "invalid until", 400)
			return
		}
		q += fmt.Sprintf(" AND occurred_at <= $%d", len(args)+1)
		args = append(args, t)
	}
	q += " ORDER BY occurred_at DESC LIMIT 200"

	rows, err := s.pool.Query(r.Context(), q, args...)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	out := []AuditRow{}
	for rows.Next() {
		var a AuditRow
		var details []byte
		if err := rows.Scan(&a.ID, &a.TenantID, &a.WorkspaceID, &a.Actor, &a.Action, &a.Resource, &a.Outcome, &details, &a.CorrelationID, &a.PrevHash, &a.Hash, &a.OccurredAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.Unmarshal(details, &a.Details)
		out = append(out, a)
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *queryServer) verifyAuditChain(w http.ResponseWriter, r *http.Request) {
	tenantStr := r.URL.Query().Get("tenant_id")
	if tenantStr == "" {
		http.Error(w, "tenant_id is required", 400)
		return
	}
	tid, err := uuid.Parse(tenantStr)
	if err != nil {
		http.Error(w, "invalid tenant_id", 400)
		return
	}

	var result VerifyResult
	result.TenantID = tid
	err = s.pool.QueryRow(r.Context(), `
		WITH ordered AS (
		  SELECT
		    tenant_id,
		    workspace_id,
		    actor,
		    action,
		    resource,
		    outcome,
		    details,
		    correlation_id,
		    occurred_at,
		    prev_hash,
		    hash,
		    COALESCE(LAG(hash) OVER (ORDER BY occurred_at, id), repeat('0', 64)) AS expected_prev_hash
		  FROM audit_event
		  WHERE tenant_id = $1
		), checked AS (
		  SELECT
		    prev_hash = expected_prev_hash
		    AND hash = encode(digest(concat_ws('|',
		      tenant_id::text,
		      coalesce(workspace_id::text, ''),
		      actor,
		      action,
		      resource,
		      outcome,
		      details::text,
		      coalesce(correlation_id, ''),
		      occurred_at::text,
		      prev_hash), 'sha256'), 'hex') AS valid
		  FROM ordered
		)
		SELECT COUNT(*)::int, COUNT(*) FILTER (WHERE NOT valid)::int FROM checked`, tid).
		Scan(&result.CheckedRows, &result.InvalidRows)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	result.ChainIsValid = result.InvalidRows == 0

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
