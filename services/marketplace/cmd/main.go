package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/marketplace/internal/marketplace"
)

func main() {
	svc := marketplace.NewService(marketplace.LogSink{})
	mux := http.NewServeMux()
	marketplace.NewHandler(svc).Mount(mux)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8095"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("marketplace listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
