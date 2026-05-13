// Package server wires the gateway's HTTP surface: middleware, routing,
// ingress hardening, auth and handlers.
package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/forge-eng-fabric/services/skill-gateway/internal/auth"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/events"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/packagestore"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/ratelimit"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/registry"
)

// Config is the bag of dependencies a server needs.
type Config struct {
	Addr            string
	Pool            *pgxpool.Pool
	Redis           *redis.Client
	Kafka           *kgo.Client
	EventsTopic     string
	Registry        *registry.Client
	PackageStore    packagestore.Store
	AllowedOrigins  []string
	BodyLimitBytes  int64
	RateCapacity    int
	RateWindow      time.Duration
}

// Server is the gateway HTTP server.
type Server struct {
	cfg     Config
	auth    *auth.Service
	events  *events.Producer
	limiter ratelimit.Limiter
}

// New constructs the server with sane defaults.
func New(cfg Config) *Server {
	if cfg.BodyLimitBytes == 0 {
		cfg.BodyLimitBytes = 8 * 1024 * 1024
	}
	if cfg.RateCapacity == 0 {
		cfg.RateCapacity = 60
	}
	if cfg.RateWindow == 0 {
		cfg.RateWindow = time.Minute
	}
	return &Server{
		cfg:     cfg,
		auth:    auth.NewService(cfg.Pool),
		events:  &events.Producer{Client: cfg.Kafka, Topic: cfg.EventsTopic},
		limiter: ratelimit.NewRedis(cfg.Redis, ratelimit.Config{Capacity: cfg.RateCapacity, Window: cfg.RateWindow}),
	}
}

// Handler returns the configured chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(correlationID)
	r.Use(maxBytes(s.cfg.BodyLimitBytes))
	r.Use(corsAllowlist(s.cfg.AllowedOrigins))

	// Unauthenticated.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := s.cfg.Pool.Ping(r.Context()); err != nil {
			http.Error(w, "db not ready", 503)
			return
		}
		w.WriteHeader(200)
	})
	r.Route("/v1/gateway/auth", func(r chi.Router) {
		r.Post("/device", s.handleAuthDevice)
		r.Post("/token", s.handleAuthToken)
	})

	// Authenticated.
	r.Group(func(r chi.Router) {
		r.Use(s.requirePAT)
		r.Use(s.requireRate)
		r.Route("/v1/gateway", func(r chi.Router) {
			r.Get("/assets", s.handleListAssets)
			r.Get("/assets/{assetID}/versions/{version}/package", s.handleDownloadPackage)
			r.Post("/tokens", s.handleIssueToken)
			r.Delete("/tokens/{tokenID}", s.handleRevokeToken)
			r.HandleFunc("/mcp/{assetID}/*", s.handleMCPProxy)
			r.HandleFunc("/mcp/{assetID}", s.handleMCPProxy)
			r.Post("/a2a/{assetID}", s.handleA2A)
		})
	})

	return otelhttp.NewHandler(r, "skill-gateway")
}

// --- middleware -------------------------------------------------------

type ctxKey int

const (
	cidKey ctxKey = iota
	patKey
)

func correlationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Forge-Correlation-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cidKey, id)))
	})
}

func maxBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func corsAllowlist(allowed []string) func(http.Handler) http.Handler {
	set := map[string]struct{}{}
	for _, o := range allowed {
		set[strings.TrimRight(o, "/")] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := set[strings.TrimRight(origin, "/")]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Headers", "authorization,content-type,x-correlation-id")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) requirePAT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			http.Error(w, "missing bearer token", 401)
			return
		}
		token := strings.TrimSpace(header[7:])
		pat, err := s.auth.Lookup(r.Context(), token)
		if err != nil {
			http.Error(w, err.Error(), 401)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), patKey, pat)))
	})
}

func (s *Server) requireRate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pat, _ := r.Context().Value(patKey).(*auth.PAT)
		key := "rl:"
		if pat != nil {
			key += pat.ID.String()
		} else {
			key += r.RemoteAddr
		}
		ok, retry, err := s.limiter.Allow(r.Context(), key)
		if err != nil {
			next.ServeHTTP(w, r) // fail-open to avoid wedging the gateway on a Redis outage
			return
		}
		if !ok {
			w.Header().Set("Retry-After", itoa(retry))
			http.Error(w, "rate_limited", 429)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func itoa(i int) string {
	if i <= 0 {
		return "1"
	}
	return strings_FormatInt(i)
}

// tiny helper to avoid importing strconv just for the formatting case above.
func strings_FormatInt(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// patFromContext extracts the verified PAT or nil.
func patFromContext(r *http.Request) *auth.PAT {
	v, _ := r.Context().Value(patKey).(*auth.PAT)
	return v
}

// correlationFromContext returns the request's correlation id.
func correlationFromContext(r *http.Request) string {
	v, _ := r.Context().Value(cidKey).(string)
	return v
}
