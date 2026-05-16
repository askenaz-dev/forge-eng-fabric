// prompt-template-service renders versioned prompt-template assets for
// workflow-runtime's LLM step executor. See ai-flow-authoring change,
// llm-flow-node capability.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/forge-eng-fabric/services/prompt-template-service/internal/prompts"
)

func main() {
	srv := &prompts.Server{Renderer: prompts.NewStubRenderer()}
	addr := os.Getenv("PROMPT_TEMPLATE_ADDR")
	if addr == "" {
		addr = ":8099"
	}
	log.Printf("prompt-template-service: listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
