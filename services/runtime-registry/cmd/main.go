package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/runtime-registry/internal/runtime"
)

func main() {
	store := runtime.NewStore()
	svc := runtime.NewService(store, runtime.LogSink{})

	// FORGE_BOOTSTRAP_TENANT primes the Terraform state-backend list for a
	// tenant so it can request Provisioned runtimes immediately.
	if seedTenant := os.Getenv("FORGE_BOOTSTRAP_TENANT"); seedTenant != "" {
		if b, ok := svc.Backends.(*runtime.InMemoryBackends); ok {
			b.Bootstrap(seedTenant)
		}
	}

	// Seed a shared default runtime if requested. The default runtime is
	// visible to every workspace in its tenant (Visibility=tenant) so new
	// installs have something deployable on day one without going through
	// BYO onboarding. Disabled when FORGE_DEFAULT_RUNTIME_TENANT is empty.
	if defaultTenant := os.Getenv("FORGE_DEFAULT_RUNTIME_TENANT"); defaultTenant != "" {
		seedSharedRuntime(store, defaultTenant)
	}

	mux := http.NewServeMux()
	runtime.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8110"
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("runtime-registry listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}

// seedSharedRuntime inserts a Forge-managed runtime visible to every workspace
// in `tenantID`. Idempotent: re-seeding finds the existing row by name and
// skips. The runtime is Minikube/Provisioned in dev so it works without GCP
// credentials; production seeds should override via env to point at the
// platform's shared cluster.
func seedSharedRuntime(store *runtime.Store, tenantID string) {
	name := os.Getenv("FORGE_DEFAULT_RUNTIME_NAME")
	if name == "" {
		name = "Forge shared (dev)"
	}
	rType := runtime.Type(os.Getenv("FORGE_DEFAULT_RUNTIME_TYPE"))
	switch rType {
	case runtime.TypeGKE, runtime.TypeCloudRun, runtime.TypeMinikube:
		// ok
	default:
		rType = runtime.TypeMinikube
	}
	region := os.Getenv("FORGE_DEFAULT_RUNTIME_REGION")
	if region == "" {
		region = "local"
	}
	for _, existing := range store.List("") {
		if existing.TenantID == tenantID && existing.Name == name {
			return // already seeded
		}
	}
	r := &runtime.Runtime{
		TenantID:     tenantID,
		WorkspaceID:  "forge-platform",
		Type:         rType,
		Mode:         runtime.ModeProvisioned,
		Visibility:   runtime.VisibilityTenant,
		Name:         name,
		Region:       region,
		Namespace:    "default",
		Capabilities: runtime.DefaultCapabilities(rType),
		Status:       "ready",
		Labels: map[string]any{
			"managed_by": "forge",
			"shared":     true,
		},
	}
	if err := store.Insert(r); err != nil {
		log.Printf("seed shared runtime: %v", err)
		return
	}
	log.Printf("seeded shared runtime %q for tenant %q", name, tenantID)
}
