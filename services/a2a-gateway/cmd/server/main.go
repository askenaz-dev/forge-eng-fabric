// Package main is the entry point for services/a2a-gateway. The gateway
// fronts every approved A2A agent — internal (registry-owned) and
// external (third-party endpoints registered per Tenant). Two flows
// share the same handler:
//
//   * Outbound: an internal caller (workflow-runtime, runner, alfred)
//     uses JSON-RPC over HTTP+SSE to invoke an external partner; the
//     gateway brokers the partner credential and signs the identity
//     headers it forwards.
//
//   * Inbound: an enrolled external partner uses HMAC-signed bodies to
//     invoke a Forge agent; the gateway authenticates the partner,
//     translates the identity into principal_kind=external_agent, and
//     routes to the agent runtime.
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
	partners := NewPartnerStore()

	srv := &server{
		cfg:         cfg,
		km:          km,
		metrics:     metrics,
		registry:    registry,
		policy:      policy,
		budget:      budget,
		secrets:     secrets,
		relay:       relay,
		rateLimiter: rateLimiter,
		publisher:   publisher,
		partners:    partners,
	}

	mux := buildRouter(srv)
	httpSrv := &http.Server{Addr: cfg.Addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("a2a-gateway listening on %s", cfg.Addr)
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

type server struct {
	cfg         config
	km          *KeyManager
	metrics     *Metrics
	registry    RegistryClient
	policy      PolicyClient
	budget      BudgetClient
	secrets     SecretFetcher
	relay       *Relay
	rateLimiter RateLimiter
	publisher   Publisher
	partners    *PartnerStore
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
		Addr:             get("ADDR", ":8093"),
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

// ctx keys ---

type ctxKey int

const (
	ctxKeyPrincipal ctxKey = iota
	ctxKeyTenant
	ctxKeyWorkspace
	ctxKeyCorrelationID
)

type urlParamKey string

func urlParam(r *http.Request, name string) string {
	v, _ := r.Context().Value(urlParamKey(name)).(string)
	return v
}

// router ---

type route struct {
	method  string
	pattern string
	handler http.HandlerFunc
	auth    bool
}

func buildRouter(s *server) http.Handler {
	routes := []route{
		{http.MethodGet, "/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }, false},
		{http.MethodGet, "/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }, false},
		{http.MethodGet, "/metrics", s.metrics.handler(), false},
		{http.MethodGet, "/jwks", s.jwksHandler, false},
		{http.MethodGet, "/v1/gw/a2a/catalog", s.catalogHandler, true},
		{http.MethodGet, "/v1/gw/a2a/partners", s.partnersHandler, true},
		{http.MethodPost, "/v1/gw/a2a/partners", s.partnersHandler, true},
		{http.MethodPost, "/v1/gw/a2a/{assetID}", s.invokeHandler, true},
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
				if rt.auth {
					s.requireIdentity(rt.handler).ServeHTTP(w, r.WithContext(ctx))
				} else {
					rt.handler.ServeHTTP(w, r.WithContext(ctx))
				}
				return
			}
		}
		http.Error(w, "not found", 404)
	})
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

// requireIdentity mirrors the mcp-gateway pattern. For inbound A2A calls
// the partner-auth header is present and the request bypasses this
// middleware via the route table; for outbound calls + partner-management
// + catalog, the Bearer JWT is required.
func (s *server) requireIdentity(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inbound A2A requests carry partner auth, not a Bearer token —
		// the inbound handler does its own auth via the partner store.
		// We still inject a correlation id so all paths emit it.
		if r.Header.Get(HeaderPartnerAuth) != "" {
			cid := r.Header.Get(HeaderCorrelationID)
			if cid == "" {
				cid = newUUID()
			}
			ctx := context.WithValue(r.Context(), ctxKeyCorrelationID, cid)
			w.Header().Set(HeaderCorrelationID, cid)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		tok := stripBearerScheme(r.Header.Get("authorization"))
		ctx := r.Context()
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

func parseIdentityToken(tok string) (principal, tenant, workspace string, ok bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
