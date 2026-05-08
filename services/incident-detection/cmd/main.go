package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/incident-detection/internal/detection"
)

func main() {
	store := detection.NewStore()
	svc := detection.NewService(store, detection.LogSink{})
	mux := http.NewServeMux()
	detection.NewHandler(svc).Mount(mux)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8101"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("incident-detection listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
