package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/sdlc-orchestrator/internal/sdlc"
)

func main() {
	store := sdlc.NewStore()
	svc := sdlc.NewService(store, sdlc.LogSink{})

	mux := http.NewServeMux()
	sdlc.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8089"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("sdlc-orchestrator listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
