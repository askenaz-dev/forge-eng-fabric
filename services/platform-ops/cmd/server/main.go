// platform-ops is Alfred's exclusive action surface for autonomous platform operations.
// Generic shell access (kubectl, psql, docker) is NEVER exposed; each action is a
// dedicated semantic endpoint with OPA pre-check, post-validate probe, and audit row.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forge-eng-fabric/services/platform-ops/internal/audit"
	"github.com/forge-eng-fabric/services/platform-ops/internal/opa"
	"github.com/forge-eng-fabric/services/platform-ops/internal/server"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	addr := env("ADDR", ":8130")
	postgresURL := env("POSTGRES_URL", "postgres://forge:forge@localhost:15432/forge?sslmode=disable")
	policyDir := env("POLICY_DIR", "../../policies/alfred")

	pool, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	opaClient, err := opa.New(policyDir)
	if err != nil {
		log.Fatalf("opa: %v", err)
	}

	slog.Info("platform-ops started",
		"addr", addr,
		"policy_dir", policyDir,
		"bundle_hash", opaClient.BundleHash(),
	)

	srv := server.New(server.Config{
		Addr:      addr,
		Pool:      pool,
		OPA:       opaClient,
		Audit:     audit.New(pool),
		PolicyDir: policyDir,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
	<-done
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
