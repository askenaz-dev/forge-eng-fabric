// Audit service: consumes forge.events, persists to audit_event with
// per-tenant prev_hash chain (DB trigger), and exposes a query API.
package main

import (
	"context"
	"encoding/json"
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
	})

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: r, ReadHeaderTimeout: 10 * time.Second}
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
		PostgresURL:      get("POSTGRES_URL", "postgres://forge:forge@localhost:5432/forge_audit?sslmode=disable"),
		KeycloakIssuer:   get("KEYCLOAK_ISSUER", "http://localhost:8080/realms/forge"),
		KeycloakAudience: get("KEYCLOAK_AUDIENCE", "forge-control-plane"),
		KafkaBrokers:     get("KAFKA_BROKERS", "localhost:29092"),
		EventsTopic:      get("EVENTS_TOPIC", "forge.events"),
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
	rows, err := s.pool.Query(r.Context(),
		`SELECT id,tenant_id,workspace_id,actor,action,resource,outcome,details,COALESCE(correlation_id,''),prev_hash,hash,occurred_at
		 FROM audit_event WHERE tenant_id=$1 ORDER BY occurred_at DESC LIMIT 200`, tid)
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
