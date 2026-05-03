package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/policy-engine/internal/policy"
)

func main() {
	engine := policy.DefaultEngine()
	if path := os.Getenv("POLICY_FILE"); path != "" {
		file, err := os.Open(path)
		if err != nil {
			log.Fatalf("open policy file: %v", err)
		}
		defer file.Close()
		engine, err = policy.LoadYAML(file)
		if err != nil {
			log.Fatalf("load policy file: %v", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /v1/evaluate", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req policy.EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := engine.Evaluate(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8084"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("policy-engine listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
