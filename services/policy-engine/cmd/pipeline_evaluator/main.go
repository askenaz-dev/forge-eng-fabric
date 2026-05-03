package main

import (
    "encoding/json"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/forge-eng-fabric/services/policy-engine/internal/policy"
)

func main() {
    // Lightweight HTTP wrapper that accepts pipeline.gate.evaluated events and
    // responds with policy decisions using the existing policy engine.
    engine := policy.DefaultEngine()
    // If a rules file exists, load it
    if path := os.Getenv("POLICY_RULES"); path != "" {
        file, err := os.Open(path)
        if err == nil {
            if eng, err := policy.LoadYAML(file); err == nil {
                engine = eng
            }
            file.Close()
        }
    }
    http.HandleFunc("/v1/evaluate-pipeline", func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        defer r.Body.Close()
        var event map[string]any
        if err := json.Unmarshal(body, &event); err != nil {
            http.Error(w, "bad payload", 400)
            return
        }
        resp, err := engine.EvaluatePipelineGate(event)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        w.Header().Set("content-type", "application/json")
        _ = json.NewEncoder(w).Encode(resp)
    })
    addr := ":8086"
    log.Printf("pipeline-evaluator listening on %s", addr)
    srv := &http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second}
    log.Fatal(srv.ListenAndServe())
}
