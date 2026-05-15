package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/sdlc-iac/internal/iac"
)

func main() {
	skills := iac.NewIaCSkills(iac.LogSink{})
	mux := http.NewServeMux()
	iac.NewHandler(skills).Mount(mux)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8110"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("sdlc-iac listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
