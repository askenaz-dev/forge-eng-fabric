package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/forge-eng-fabric/services/platform-ops/internal/audit"
	"github.com/forge-eng-fabric/services/platform-ops/internal/github"
)

// POST /v1/code-fixes/open-pr
//
// Opens a pull request on the named repo with the provided patches.
// This endpoint NEVER merges — it only proposes changes for human review.
type CodeFixRequest struct {
	Repo          string       `json:"repo"`            // "owner/repo" or just "repo" (uses GITHUB_OWNER)
	Branch        string       `json:"branch"`          // feature branch to create
	CommitMessage string       `json:"commit_message"`
	PRTitle       string       `json:"pr_title"`
	PRBody        string       `json:"pr_body,omitempty"`
	Files         []FilePatch  `json:"files"`           // list of file patches to apply
	ExpectedOutcome string     `json:"expected_outcome,omitempty"` // "ci_green" etc.
	SymptomID     string       `json:"symptom_id,omitempty"`
	SessionID     string       `json:"session_id,omitempty"`
}

// FilePatch describes a single file to create or update in the PR branch.
type FilePatch struct {
	Path    string `json:"path"`    // repo-relative file path
	Content string `json:"content"` // full file content (base64 if is_binary=true)
	IsBinary bool  `json:"is_binary,omitempty"`
}

func (h *handler) codeFixOpenPR(w http.ResponseWriter, r *http.Request) {
	var body CodeFixRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields.
	if body.Repo == "" {
		writeError(w, http.StatusBadRequest, "repo is required")
		return
	}
	if len(body.Files) == 0 {
		writeError(w, http.StatusBadRequest, "files must not be empty")
		return
	}
	if body.Branch == "" {
		writeError(w, http.StatusBadRequest, "branch is required")
		return
	}
	if body.PRTitle == "" {
		writeError(w, http.StatusBadRequest, "pr_title is required")
		return
	}
	if body.CommitMessage == "" {
		writeError(w, http.StatusBadRequest, "commit_message is required")
		return
	}

	// OPA pre-check.
	decision, _, _, _ := h.cfg.OPA.EvalRiskClassifier(r.Context(), map[string]any{
		"action_class":  "mutate-code",
		"blast_radius":  "repository",
		"reversibility": "easy",
		"scope":         "local",
		"target":        body.Repo,
		"actor":         r.Header.Get("X-Forge-Actor"),
	})
	if decision == "deny" {
		writeError(w, http.StatusForbidden, "OPA denied code-fix action")
		return
	}

	// Parse owner/repo from the Repo field.
	owner, repo := parseRepo(body.Repo)

	gh := github.New(github.Config{
		Token: os.Getenv("GITHUB_TOKEN"),
		Owner: owner,
		Repo:  repo,
	})

	// Get main SHA, create branch, commit each file, open PR.
	pr, err := gh.OpenCodeFixPR(r.Context(), github.CodeFixPRInput{
		Branch:        body.Branch,
		CommitMessage: body.CommitMessage,
		PRTitle:       body.PRTitle,
		PRBody:        appendNeverMergeNotice(body.PRBody, body.SymptomID, body.SessionID),
		Files:         convertFiles(body.Files),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open PR: "+err.Error())
		return
	}

	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:          actionID,
		Actor:            r.Header.Get("X-Forge-Actor"),
		Action:           "platform-ops:code-fix:open-pr",
		Target:           body.Repo,
		Outcome:          "success",
		SymptomID:        body.SymptomID,
		SessionID:        body.SessionID,
		PolicyBundleHash: h.cfg.OPA.BundleHash(),
		Details: map[string]any{
			"pr_url":           pr.HTMLURL,
			"pr_number":        pr.Number,
			"branch":           body.Branch,
			"files_count":      len(body.Files),
			"expected_outcome": body.ExpectedOutcome,
		},
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"audit_id":         actionID,
		"pr_url":           pr.HTMLURL,
		"pr_number":        pr.Number,
		"repo":             fmt.Sprintf("%s/%s", owner, repo),
		"branch":           body.Branch,
		"outcome":          "success",
		"never_merges":     true,
		"expected_outcome": body.ExpectedOutcome,
		"timestamp":        time.Now().UTC(),
	})
}

// parseRepo splits "owner/repo" or returns (GITHUB_OWNER env, repo).
func parseRepo(s string) (owner, repo string) {
	for i, c := range s {
		if c == '/' {
			return s[:i], s[i+1:]
		}
	}
	return env("GITHUB_OWNER", "forge-eng-fabric"), s
}

func appendNeverMergeNotice(body, symptomID, sessionID string) string {
	notice := "\n\n---\n> ⚠️ **Opened automatically by Alfred (platform-ops).** " +
		"This PR must be reviewed and merged by a human — Alfred never self-merges."
	if symptomID != "" {
		notice += fmt.Sprintf("\n> **Symptom ID:** `%s`", symptomID)
	}
	if sessionID != "" {
		notice += fmt.Sprintf("\n> **Session ID:** `%s`", sessionID)
	}
	return body + notice
}

func convertFiles(files []FilePatch) []github.FilePatch {
	out := make([]github.FilePatch, len(files))
	for i, f := range files {
		content := f.Content
		if f.IsBinary {
			// Already base64; decode to bytes then re-encode uniformly.
			if b, err := base64.StdEncoding.DecodeString(f.Content); err == nil {
				content = string(b)
			}
		}
		out[i] = github.FilePatch{Path: f.Path, Content: content}
	}
	return out
}
