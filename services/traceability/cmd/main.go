package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/traceability/internal/traceability"
)

func main() {
	store := traceability.NewStore()
	svc := traceability.NewService(store, traceability.LogSink{})
	stop := make(chan struct{})
	defer close(stop)
	go svc.RunMaterializedViewRefresher(5*time.Minute, stop)

	mux := http.NewServeMux()
	traceability.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8090"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("traceability listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
