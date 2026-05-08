package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/workflow-registry/internal/registry"
)

func main() {
	svc := registry.NewService(registry.LogSink{})
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
