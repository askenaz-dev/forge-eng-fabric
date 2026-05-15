package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/application/internal/application"
)

func main() {
	store := application.NewStore()
	authz := application.NewAllowAllAuthorizer()
	events := application.NewMemorySink()
	audit := application.NewMemoryAuditSink()
	live := application.NoLiveArtefacts{}
	dir := application.StaticWorkspaceLookup{}
	svc := application.NewService(store, authz, events, audit, live, dir)

	mux := http.NewServeMux()
	application.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8095"
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("application service listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
