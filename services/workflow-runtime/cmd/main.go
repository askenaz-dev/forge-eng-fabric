package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/workflow-runtime/internal/runtime"
)

func main() {
	engine := runtime.NewInMemoryEngine(runtime.NewActivityRegistry(nil), runtime.LogSink{})
	svc := runtime.NewService(engine, nil)

	mux := http.NewServeMux()
	runtime.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8093"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("workflow-runtime listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
