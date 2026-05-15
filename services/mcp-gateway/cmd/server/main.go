// Package main is the entry point for services/mcp-gateway. The gateway
// is the only internal seam through which Forge runners, Alfred and the
// public skill-gateway reach an MCP server. Every call goes through:
//
//	rate-limit → registry-lookup → policy → budget → identity-sign →
//	(optional credential broker for external) → relay (HTTP or SSE) →
//	audit + invocation event
//
// The service is stateless; horizontal-scale-out is handled by k8s HPA.
// Public-edge traffic enters via services/skill-gateway with PAT auth;
// internal traffic carries workload-identity JWTs.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/forge-eng-fabric/services/mcp-gateway/internal/drift"
)

func main() {
	cfg := loadConfig()
	km, err := NewKeyManager(cfg.IdentityRotation)
	if err != nil {
		log.Fatalf("identity: %v", err)
	}
	driftCtx, driftCancel := context.WithCancel(context.Background())
	defer driftCancel()
	go km.Start(driftCtx)

	metrics := newMetrics()
	relay := newRelay()
	registry := newHTTPRegistryClient(cfg.RegistryURL, cfg.RegistryToken)
	policy := newHTTPPolicyClient(cfg.PolicyEngineURL)
	budget := newHTTPBudgetClient(cfg.BudgetURL)
	secrets := envFileSecretFetcher{}
	publisher := newPublisher(cfg.KafkaBrokers, cfg.EventsTopic)
	rateLimiter := buildRateLimiter(cfg)

	driftRegistry := drift.NewHTTPRegistryClient(cfg.RegistryURL, cfg.RegistryToken)
	driftDetector := drift.New(driftRegistry, publisher)

	srv := &server{
		cfg:           cfg,
		km:            km,
		metrics:       metrics,
		registry:      registry,
		policy:        policy,
		budget:        budget,
		secrets:       secrets,
		relay:         relay,
		rateLimiter:   rateLimiter,
		publisher:     publisher,
		driftDetector: driftDetector,
	}

	mux := buildRouter(srv)
	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("mcp-gateway listening on %s", cfg.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	driftCancel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

// server holds the wired collaborators. Tests construct one of these
// with stubbed clients and call into the handler functions directly.
type server struct {
	cfg          config
	km           *KeyManager
	metrics      *Metrics
	registry     RegistryClient
	policy       PolicyClient
	budget       BudgetClient
	secrets      SecretFetcher
	relay        *Relay
	rateLimiter  RateLimiter
	publisher    Publisher
	driftDetector *drift.Detector
}

type config struct {
	Addr             string
	RegistryURL      string
	RegistryToken    string
	PolicyEngineURL  string
	BudgetURL        string
	KafkaBrokers     string
	EventsTopic      string
	RedisAddr        string
	RateLimitPerMin  int
	IdentityRotation time.Duration
	SSEBufferSize    int
	Environment      string
}

func loadConfig() config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	getInt := func(k string, def int) int {
		v := os.Getenv(k)
		if v == "" {
			return def
		}
		var out int
		fmt.Sscanf(v, "%d", &out)
		if out <= 0 {
			return def
		}
		return out
	}
	rotation, err := time.ParseDuration(get("IDENTITY_ROTATION", "24h"))
	if err != nil {
		rotation = 24 * time.Hour
	}
	return config{
		Addr:             get("ADDR", ":8092"),
		RegistryURL:      get("REGISTRY_URL", "http://registry:8082"),
		RegistryToken:    get("REGISTRY_TOKEN", ""),
		PolicyEngineURL:  get("POLICY_ENGINE_URL", ""),
		BudgetURL:        get("BUDGET_URL", ""),
		KafkaBrokers:     get("KAFKA_BROKERS", "kafka:9092"),
		EventsTopic:      get("EVENTS_TOPIC", "forge.events"),
		RedisAddr:        get("REDIS_ADDR", "redis:6379"),
		RateLimitPerMin:  getInt("RATE_LIMIT_PER_MIN", 600),
		IdentityRotation: rotation,
		SSEBufferSize:    getInt("SSE_BUFFER_SIZE", 32),
		Environment:      get("ENV", "local"),
	}
}

func buildRateLimiter(cfg config) RateLimiter {
	if cfg.RedisAddr == "" || strings.EqualFold(cfg.RedisAddr, "disabled") {
		return newInMemoryRateLimiter(cfg.RateLimitPerMin, time.Minute)
	}
	return newRedisRateLimiter(cfg.RedisAddr, cfg.RateLimitPerMin, time.Minute)
}

// ctxKey types ----------------------------------------------------------

type ctxKey int

const (
	ctxKeyPrincipal ctxKey = iota
	ctxKeyTenant
	ctxKeyWorkspace
	ctxKeyCorrelationID
)

// router + lightweight URL-param routing --------------------------------

// We avoid pulling chi into this module to keep the dep graph small —
// the only routes are /v1/gw/mcp/{asset_id} and /v1/gw/mcp/catalog plus
// health / metrics / jwks. A tiny custom muxer covers them.

type route struct {
	method  string
	pattern string
	handler http.HandlerFunc
}

func buildRouter(s *server) http.Handler {
	routes := []route{
		{http.MethodGet, "/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }},
		{http.MethodGet, "/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }},
		{http.MethodGet, "/metrics", s.metrics.handler()},
		{http.MethodGet, "/jwks", s.jwksHandler},
		{http.MethodGet, "/v1/gw/mcp/catalog", chainMiddleware(s.catalogHandler, s.requireIdentity)},
		{http.MethodPost, "/v1/gw/mcp/{assetID}", chainMiddleware(s.invokeHandler, s.requireIdentity)},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, rt := range routes {
			if rt.method != r.Method {
				continue
			}
			if vars, ok := matchPattern(rt.pattern, r.URL.Path); ok {
				ctx := r.Context()
				for k, v := range vars {
					ctx = context.WithValue(ctx, urlParamKey(k), v)
				}
				rt.handler.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		http.Error(w, "not found", 404)
	})
}

type urlParamKey string

// chiURLParam mirrors the chi.URLParam signature used by other Forge
// services, but reads from our context-stored variables.
func chiURLParam(r *http.Request, name string) string {
	v, _ := r.Context().Value(urlParamKey(name)).(string)
	return v
}

func matchPattern(pattern, path string) (map[string]string, bool) {
	pParts := strings.Split(strings.Trim(pattern, "/"), "/")
	uParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pParts) != len(uParts) {
		return nil, false
	}
	out := map[string]string{}
	for i, p := range pParts {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			out[p[1:len(p)-1]] = uParts[i]
			continue
		}
		if p != uParts[i] {
			return nil, false
		}
	}
	return out, true
}

func chainMiddleware(h http.HandlerFunc, mws ...func(http.Handler) http.Handler) http.HandlerFunc {
	var hh http.Handler = h
	for i := len(mws) - 1; i >= 0; i-- {
		hh = mws[i](hh)
	}
	return hh.ServeHTTP
}

// requireIdentity is the inbound auth middleware. Internal callers carry
// a workload-identity JWT in the standard Bearer scheme; the gateway
// extracts the principal / tenant / workspace claims and propagates them
// through the request context for handlers + the identity-signing step.
//
// For local dev / tests, the caller may also pass the identity directly
// via X-Forge-Principal / X-Forge-Tenant / X-Forge-Workspace headers.
// This shortcut is gated by ENV != "prod".
func (s *server) requireIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Bearer-token path: production. Decoded by services/skill-gateway
		// at the public edge or by the runner's SPIFFE-attested workload
		// identity for internal callers; the gateway treats the JWT as
		// trusted at this layer.
		tok := stripBearerScheme(r.Header.Get("authorization"))
		if tok != "" {
			principal, tenant, workspace, ok := parseIdentityToken(tok)
			if !ok {
				writeJSONErr(w, 401, "invalid_identity_token", "could not parse identity claims")
				return
			}
			ctx = context.WithValue(ctx, ctxKeyPrincipal, principal)
			ctx = context.WithValue(ctx, ctxKeyTenant, tenant)
			ctx = context.WithValue(ctx, ctxKeyWorkspace, workspace)
		} else if s.cfg.Environment != "prod" {
			// Dev shortcut.
			ctx = context.WithValue(ctx, ctxKeyPrincipal, r.Header.Get(HeaderPrincipal))
			ctx = context.WithValue(ctx, ctxKeyTenant, r.Header.Get(HeaderTenant))
			ctx = context.WithValue(ctx, ctxKeyWorkspace, r.Header.Get(HeaderWorkspace))
		} else {
			writeJSONErr(w, 401, "missing_identity", "Authorization: Bearer required")
			return
		}
		cid := r.Header.Get(HeaderCorrelationID)
		if cid == "" {
			cid = newUUID()
		}
		ctx = context.WithValue(ctx, ctxKeyCorrelationID, cid)
		w.Header().Set(HeaderCorrelationID, cid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// parseIdentityToken is the minimum claim extractor the gateway needs.
// In production this is wired to a real JWT validator (matching the
// pattern in services/registry); for §4 we use a plain unsigned shape
// `principal.tenant.workspace` so tests + local dev don't need a
// Keycloak round-trip. Production wiring is a follow-up task.
func parseIdentityToken(tok string) (principal, tenant, workspace string, ok bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// newUUID returns a v4 UUID using crypto/rand. We avoid github.com/google/uuid
// here so this service has zero external deps (chi, uuid, otel etc. all
// live in the existing services; we'll add them as the service grows).
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
