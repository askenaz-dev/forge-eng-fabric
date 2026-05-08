package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/iac-drift/internal/drift"
)

func main() {
	svc := drift.NewService(drift.NewStore(), drift.LogSink{})
	stop := make(chan struct{})
	defer close(stop)
	go svc.RunScheduler(1*time.Hour, stop)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8088"
	}
	log.Printf("iac-drift listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
