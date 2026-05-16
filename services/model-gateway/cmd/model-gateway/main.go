// model-gateway resolves LLM model references (gateway:model/<id>@<channel>)
// for workflow-runtime's LLM step executor. See ai-flow-authoring change,
// llm-flow-node capability.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/forge-eng-fabric/services/model-gateway/internal/gateway"
)

func main() {
	srv := &gateway.Server{Resolver: &gateway.StubResolver{}}
	addr := os.Getenv("MODEL_GATEWAY_ADDR")
	if addr == "" {
		addr = ":8098"
	}
	log.Printf("model-gateway: listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
