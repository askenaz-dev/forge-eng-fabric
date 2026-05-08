package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/healing-engine/internal/engine"
)

func main() {
	store := engine.NewStore()
	svc := engine.NewService(store, engine.LogSink{})
	mux := http.NewServeMux()
	engine.NewHandler(svc).Mount(mux)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8102"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("healing-engine listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
