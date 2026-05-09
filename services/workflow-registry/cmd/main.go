package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/workflow-registry/internal/registry"
)

func main() {
	svc := registry.NewService(registry.LogSink{})

	// Seed reference workflows from the embedded `seeds/` directory at startup.
	// Idempotent: existing workflows/versions are skipped without error.
	seedDir := os.Getenv("WORKFLOW_REGISTRY_SEED_DIR")
	if seedDir == "" {
		seedDir = "services/workflow-registry/seeds"
	}
	tenant := os.Getenv("WORKFLOW_REGISTRY_SEED_TENANT")
	if tenant == "" {
		tenant = "forge-platform"
	}
	workspace := os.Getenv("WORKFLOW_REGISTRY_SEED_WORKSPACE")
	if workspace == "" {
		workspace = "forge-platform"
	}
	if err := svc.SeedDirectory(context.Background(), seedDir, tenant, workspace, "system:seed"); err != nil {
		log.Printf("workflow-registry: seed warnings: %v", err)
	}

	mux := http.NewServeMux()
	registry.NewHandler(svc).Mount(mux)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8094"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("workflow-registry listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
