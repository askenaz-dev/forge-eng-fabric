package main

import (
    "encoding/json"
    "log"
    "net/http"
    "time"
)

type OnboardingRequest struct {
    Workspace string `json:"workspace"`
    RepoName  string `json:"repo_name"`
    Template  string `json:"template"`
}

func handleOnboarding(w http.ResponseWriter, r *http.Request) {
    var req OnboardingRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    id := time.Now().Format("20060102T150405")
    log.Printf("onboarding requested: id=%s workspace=%s repo=%s template=%s", id, req.Workspace, req.RepoName, req.Template)
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{"request_id": id, "status": "pending"})
}

func main() {
    http.HandleFunc("/v1/onboarding", handleOnboarding)
    log.Printf("app-onboarding stub listening on :8085")
    log.Fatal(http.ListenAndServe(":8085", nil))
}
