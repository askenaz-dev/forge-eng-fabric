package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/app-onboarding/internal/onboarding"
)

func main() {
	store := onboarding.NewStore()
	svc := onboarding.NewService(store, onboarding.LogSink{})

	if dir := os.Getenv("FORGE_TEMPLATES_DIR"); dir != "" {
		svc.Catalog = onboarding.FilesystemCatalog{BaseDir: dir}
	}
	if mcp := os.Getenv("FORGE_GITHUB_MCP_URL"); mcp != "" {
		svc.GitHub = onboarding.NewHTTPGitHubMCP(mcp)
	}
	if registryURL := firstNonEmpty(os.Getenv("FORGE_REGISTRY_URL"), os.Getenv("REGISTRY_URL")); registryURL != "" {
		svc.Registrar = onboarding.NewHTTPRegistryRegistrar(registryURL, os.Getenv("FORGE_REGISTRY_TOKEN"))
	}

	mux := http.NewServeMux()
	onboarding.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8085"
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("app-onboarding listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
