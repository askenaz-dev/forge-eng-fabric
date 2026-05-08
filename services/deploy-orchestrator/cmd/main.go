package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/pkg/deployers"
	"github.com/forge-eng-fabric/pkg/deployers/cloudrun"
	"github.com/forge-eng-fabric/pkg/deployers/gke"
	"github.com/forge-eng-fabric/pkg/deployers/minikube"
	"github.com/forge-eng-fabric/services/deploy-orchestrator/internal/deploy"
)

func main() {
	store := deploy.NewStore()
	registry := deployers.NewRegistry(
		minikube.New(nil),
		gke.New(nil),
		cloudrun.New(nil),
	)
	svc := deploy.NewService(store, registry, deploy.LogSink{})

	mux := http.NewServeMux()
	deploy.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8087"
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("deploy-orchestrator listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
