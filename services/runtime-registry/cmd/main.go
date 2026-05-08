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
	if seedTenant := os.Getenv("FORGE_BOOTSTRAP_TENANT"); seedTenant != "" {
		if b, ok := svc.Backends.(*runtime.InMemoryBackends); ok {
			b.Bootstrap(seedTenant)
		}
	}

	mux := http.NewServeMux()
	runtime.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8086"
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("runtime-registry listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
