package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var openspecLine = regexp.MustCompile(`(?m)^OpenSpec:\s*(?P<id>[A-Za-z0-9_\-]+)`) // capture OpenSpec: <id>

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get("X-GitHub-Event")
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if event == "pull_request" {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "bad payload", 400)
			return
		}
		action, _ := payload["action"].(string)
		pr, _ := payload["pull_request"].(map[string]any)
		if pr == nil {
			w.WriteHeader(204)
			return
		}
		bodyText, _ := pr["body"].(string)
		matches := openspecLine.FindStringSubmatch(bodyText)
		if len(matches) > 1 {
			id := matches[1]
			// Call OpenSpec service /links to append the link
			openspecURL := os.Getenv("OPENSPEC_URL")
			if openspecURL == "" {
				openspecURL = "http://localhost:8083"
			}
			// Build link payload
			repoFullName := ""
			if repo, ok := payload["repository"].(map[string]any); ok {
				if full, _ := repo["full_name"].(string); full != "" {
					repoFullName = full
				}
			}
			prURL := ""
			if url, _ := pr["html_url"].(string); url != "" {
				prURL = url
			}
			actor := "unknown"
			if user, ok := pr["user"].(map[string]any); ok {
				if login, _ := user["login"].(string); login != "" {
					actor = login
				}
			}
			link := map[string]any{"kind": "pull_request", "ref": prURL, "namespace": "pr", "metadata": map[string]any{"repo": repoFullName}}
			body := map[string]any{"actor": actor, "link": link}
			client := &http.Client{Timeout: 5 * time.Second}
			reqBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", openspecURL+"/v1/openspecs/"+id+"/links", strings.NewReader(string(reqBody)))
			req.Header.Set("content-type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("openspec link call failed: %v", err)
			} else {
				io.ReadAll(resp.Body)
				resp.Body.Close()
				log.Printf("openspec link appended for %s (status %d)", id, resp.StatusCode)
			}
		}
		// On merged PR, append decision log to the OpenSpec if linked
		if action == "closed" {
			merged, _ := pr["merged"].(bool)
			if merged {
				// For simplicity reuse the same parser to find OpenSpec id in body
				if len(matches) > 1 {
					id := matches[1]
					// Append decision log entry via /decisions (simple payload)
					openspecURL := os.Getenv("OPENSPEC_URL")
					if openspecURL == "" {
						openspecURL = "http://localhost:8083"
					}
					actor := "unknown"
					if user, ok := pr["user"].(map[string]any); ok {
						if login, _ := user["login"].(string); login != "" {
							actor = login
						}
					}
					decision := map[string]any{
						"type":      "pr_merged",
						"actor":     actor,
						"decision":  "PR merged",
						"rationale": "Merged PR linked to OpenSpec",
						"url":       prURL(pr),
						"metadata":  map[string]any{"repo": repoFullName(payload), "pr_url": prURL(pr)},
					}
					client := &http.Client{Timeout: 5 * time.Second}
					db, _ := json.Marshal(decision)
					req, _ := http.NewRequest("POST", openspecURL+"/v1/openspecs/"+id+"/decisions", strings.NewReader(string(db)))
					req.Header.Set("content-type", "application/json")
					resp, err := client.Do(req)
					if err != nil {
						log.Printf("openspec decision call failed: %v", err)
					} else {
						io.ReadAll(resp.Body)
						resp.Body.Close()
						log.Printf("openspec decision appended for %s (status %d)", id, resp.StatusCode)
					}
				}
			}
		}
	}
	w.WriteHeader(204)
}

func prURL(pr map[string]any) string {
	if url, _ := pr["html_url"].(string); url != "" {
		return url
	}
	return ""
}

func repoFullName(payload map[string]any) string {
	if repo, ok := payload["repository"].(map[string]any); ok {
		if full, _ := repo["full_name"].(string); full != "" {
			return full
		}
	}
	return ""
}

func main() {
	http.HandleFunc("/webhook", handleWebhook)
	addr := ":8090"
	if a := os.Getenv("ADDR"); a != "" {
		addr = a
	}
	log.Printf("github-webhook stub listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
