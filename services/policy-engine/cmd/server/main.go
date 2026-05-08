package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/policy-engine/internal/override"
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

	overrideMgr, err := loadOverrideManager()
	if err != nil {
		log.Fatalf("load override templates: %v", err)
	}
	sdlcGateEngine, err := loadSDLCGateEngine()
	if err != nil {
		log.Fatalf("load sdlc gate templates: %v", err)
	}
	stop := make(chan struct{})
	defer close(stop)
	go overrideMgr.RunReconciler(60*time.Second, stop)

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

	mux.HandleFunc("GET /v1/overrides/templates", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(overrideMgr.Templates())
	})
	mux.HandleFunc("POST /v1/overrides", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var in override.GrantInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ov, err := overrideMgr.Grant(in)
		if err != nil {
			status := http.StatusBadRequest
			if err == override.ErrInsufficientRole {
				status = http.StatusForbidden
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ov)
	})
	mux.HandleFunc("GET /v1/overrides", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(overrideMgr.List())
	})
	mux.HandleFunc("GET /v1/sdlc-gates/templates", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"templates": sdlcGateEngine.Templates()})
	})
	mux.HandleFunc("POST /v1/sdlc-gates/evaluate", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req policy.SDLCGateEvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := sdlcGateEngine.EvaluateSDLCGate(req)
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

func loadOverrideManager() (*override.Manager, error) {
	path := os.Getenv("OVERRIDE_TEMPLATES")
	if path == "" {
		path = "policy_templates/overrides.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Allow missing file in dev: start with empty templates.
		log.Printf("override templates not found at %s, starting empty", path)
		return override.NewManager(map[string]override.Template{}, &override.MemorySink{}), nil
	}
	tpls, err := override.LoadTemplates(data)
	if err != nil {
		return nil, err
	}
	return override.NewManager(tpls, &override.MemorySink{}), nil
}

func loadSDLCGateEngine() (*policy.SDLCGateEngine, error) {
	path := os.Getenv("SDLC_GATE_TEMPLATES")
	if path == "" {
		path = "policy_templates/sdlc_gates.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("sdlc gate templates not found at %s, starting empty", path)
		return policy.NewSDLCGateEngine(nil), nil
	}
	return policy.LoadSDLCGateTemplates(data)
}
