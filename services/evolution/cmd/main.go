package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/evolution/internal/evolution"
)

func main() {
	store := evolution.NewStore()
	svc := evolution.NewService(store, evolution.LogSink{})
	mux := http.NewServeMux()
	evolution.NewHandler(svc).Mount(mux)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8103"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("evolution listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
