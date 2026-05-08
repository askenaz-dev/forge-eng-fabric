package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/asset-observability/internal/observability"
)

func main() {
	store := observability.NewStore()
	svc := observability.NewService(store, observability.LogSink{})
	mux := http.NewServeMux()
	observability.NewHandler(svc).Mount(mux)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8096"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("asset-observability listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
