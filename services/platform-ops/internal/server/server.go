// Package server wires the platform-ops HTTP API.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forge-eng-fabric/services/platform-ops/internal/audit"
	"github.com/forge-eng-fabric/services/platform-ops/internal/opa"
)

// bundleHashMismatchTotal counts audit rows whose policy_bundle_hash did not
// match the current bundle. Exposed via /metrics for the Prometheus alert rule.
var bundleHashMismatchTotal atomic.Int64

// Config holds server dependencies.
type Config struct {
	Addr       string
	Pool       *pgxpool.Pool
	OPA        *opa.Client
	Audit      *audit.Writer
	PolicyDir  string
}

// New builds the HTTP server.
func New(cfg Config) *http.Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(structuredLogger)
	r.Use(middleware.Recoverer)

	h := &handler{cfg: cfg}

	// Health / readiness
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	// Prometheus metrics
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		mismatch := bundleHashMismatchTotal.Load()
		fmt.Fprintf(w, "# HELP platform_ops_policy_bundle_hash_mismatch_total Audit rows whose policy_bundle_hash did not match the loaded bundle.\n")
		fmt.Fprintf(w, "# TYPE platform_ops_policy_bundle_hash_mismatch_total counter\n")
		fmt.Fprintf(w, "platform_ops_policy_bundle_hash_mismatch_total %d\n", mismatch)
	})

	// Diagnostic endpoints (all read-only — iter 2)
	r.Route("/v1/diagnostics", func(r chi.Router) {
		r.Get("/probe", h.diagnosticProbe)
		r.Get("/inspect/{service}", h.diagnosticInspect)
		r.Get("/logs", h.diagnosticLogs)
	})

	// Service mutation endpoints (iter 3)
	r.Route("/v1/services/{name}", func(r chi.Router) {
		r.Post("/restart", h.serviceRestart)
		r.Post("/scale", h.serviceScale)
		r.Post("/cordon", h.serviceCordon)
		r.Post("/uncordon", h.serviceUncordon)
	})

	// Circuit-breaker admin (iter 3)
	r.Post("/v1/circuit-breakers/{fingerprint}/reset", h.circuitBreakerReset)

	// Sandbox (iter 4)
	r.Route("/v1/sandbox", func(r chi.Router) {
		r.Post("/spawn", h.sandboxSpawn)
		r.Post("/{id}/run", h.sandboxRun)
		r.Delete("/{id}", h.sandboxDestroy)
	})

	// Migrations (iter 4)
	r.Post("/v1/migrations/dry-run", h.migrationsDryRun)
	r.Post("/v1/migrations/run", h.migrationsRun)
	r.Post("/v1/migrations/rollback", h.migrationsRollback)

	// Config mutations (iter 4)
	r.Post("/v1/feature-flags/{key}/toggle", h.featureFlagToggle)
	r.Post("/v1/secrets/{key}/rotate", h.secretRotate)

	// Noise rules (iter 4)
	r.Get("/v1/noise-rules", h.noiseRuleList)
	r.Post("/v1/noise-rules/propose", h.noiseRulePropose)
	r.Post("/v1/noise-rules/{id}/approve", h.noiseRuleApprove)
	r.Post("/v1/noise-rules/{id}/revoke", h.noiseRuleRevoke)

	// Webhooks
	r.Post("/v1/webhooks/github", h.githubWebhook)

	// Code fixes (iter 5)
	r.Post("/v1/code-fixes/open-pr", h.codeFixOpenPR)

	// Infrastructure (iter 6)
	r.Post("/v1/runtimes/{id}/recreate", h.runtimeRecreate)

	return &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

type handler struct {
	cfg Config
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func structuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
