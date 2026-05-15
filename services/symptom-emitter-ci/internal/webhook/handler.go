// Package webhook receives GitHub Actions webhook events and normalises
// check_run and workflow_run failures to symptom events on forge.symptoms.v1.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Producer is the minimal interface the handler needs to publish symptom events.
type Producer interface {
	Publish(ctx context.Context, key string, value []byte) error
}

// Handler is the GitHub webhook HTTP handler.
type Handler struct {
	secret   string // GITHUB_WEBHOOK_SECRET — empty disables sig check
	producer Producer
}

// New creates a Handler.
func New(secret string, p Producer) *Handler {
	return &Handler{secret: secret, producer: p}
}

// ServeHTTP handles incoming GitHub webhook POST requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	if h.secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(rawBody, h.secret, sig) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	event := r.Header.Get("X-GitHub-Event")
	switch event {
	case "check_run":
		h.handleCheckRun(r.Context(), rawBody)
	case "workflow_run":
		h.handleWorkflowRun(r.Context(), rawBody)
	default:
		// Unknown event type — accept but ignore.
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── check_run ────────────────────────────────────────────────────────────────

type checkRunPayload struct {
	Action   string `json:"action"`
	CheckRun struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HTMLURL    string `json:"html_url"`
		HeadSHA    string `json:"head_sha"`
	} `json:"check_run"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (h *Handler) handleCheckRun(ctx context.Context, body []byte) {
	var p checkRunPayload
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Warn("check_run unmarshal", "err", err)
		return
	}
	if p.Action != "completed" {
		return
	}
	if p.CheckRun.Conclusion != "failure" && p.CheckRun.Conclusion != "timed_out" {
		return
	}

	repo := p.Repository.FullName
	checkName := p.CheckRun.Name
	fingerprint := buildFingerprint(map[string]string{
		"service": repoToService(repo),
		"signal":  "ci-check-failed",
		"route":   checkName,
	})

	evt := symptomEvent{
		SymptomID:      uuid.NewString(),
		Fingerprint:    fingerprint,
		Signal:         "ci-check-failed",
		Service:        repoToService(repo),
		Severity:       "warning",
		Emitter:        "symptom-emitter-ci",
		ObservedAt:     time.Now().UTC().Format(time.RFC3339),
		SchemaVersion:  "v1",
		EvidenceExcerpt: fmt.Sprintf("check_run %q concluded=%s sha=%s",
			checkName, p.CheckRun.Conclusion, shortSHA(p.CheckRun.HeadSHA)),
		EvidenceRef: p.CheckRun.HTMLURL,
	}
	h.publish(ctx, fingerprint, evt)
}

// ── workflow_run ─────────────────────────────────────────────────────────────

type workflowRunPayload struct {
	Action      string `json:"action"`
	WorkflowRun struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HTMLURL    string `json:"html_url"`
		HeadSHA    string `json:"head_sha"`
	} `json:"workflow_run"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (h *Handler) handleWorkflowRun(ctx context.Context, body []byte) {
	var p workflowRunPayload
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Warn("workflow_run unmarshal", "err", err)
		return
	}
	if p.Action != "completed" {
		return
	}
	if p.WorkflowRun.Conclusion != "failure" && p.WorkflowRun.Conclusion != "timed_out" {
		return
	}

	repo := p.Repository.FullName
	fingerprint := buildFingerprint(map[string]string{
		"service": repoToService(repo),
		"signal":  "ci-workflow-failed",
		"route":   p.WorkflowRun.Name,
	})

	evt := symptomEvent{
		SymptomID:      uuid.NewString(),
		Fingerprint:    fingerprint,
		Signal:         "ci-workflow-failed",
		Service:        repoToService(repo),
		Severity:       "warning",
		Emitter:        "symptom-emitter-ci",
		ObservedAt:     time.Now().UTC().Format(time.RFC3339),
		SchemaVersion:  "v1",
		EvidenceExcerpt: fmt.Sprintf("workflow_run %q concluded=%s sha=%s",
			p.WorkflowRun.Name, p.WorkflowRun.Conclusion, shortSHA(p.WorkflowRun.HeadSHA)),
		EvidenceRef: p.WorkflowRun.HTMLURL,
	}
	h.publish(ctx, fingerprint, evt)
}

// ── helpers ───────────────────────────────────────────────────────────────────

type symptomEvent struct {
	SymptomID       string `json:"symptom_id"`
	Fingerprint     string `json:"fingerprint"`
	Signal          string `json:"signal"`
	Service         string `json:"service"`
	Severity        string `json:"severity"`
	Emitter         string `json:"emitter"`
	ObservedAt      string `json:"observed_at"`
	SchemaVersion   string `json:"schema_version"`
	EvidenceExcerpt string `json:"evidence_excerpt"`
	EvidenceRef     string `json:"evidence_ref,omitempty"`
}

func (h *Handler) publish(ctx context.Context, fingerprint string, evt symptomEvent) {
	b, err := json.Marshal(evt)
	if err != nil {
		slog.Error("marshal symptom event", "err", err)
		return
	}
	if err := h.producer.Publish(ctx, fingerprint, b); err != nil {
		slog.Error("publish symptom event", "fingerprint", fingerprint, "err", err)
		return
	}
	slog.Info("symptom emitted",
		"fingerprint", fingerprint,
		"signal", evt.Signal,
		"symptom_id", evt.SymptomID,
	)
}

// buildFingerprint builds a canonical fingerprint from a dimension map.
// Dimensions are sorted alphabetically and joined with |.
func buildFingerprint(dims map[string]string) string {
	keys := make([]string, 0, len(dims))
	for k := range dims {
		keys = append(keys, k)
	}
	// Sort keys deterministically (insertion order not guaranteed).
	sortedKeys := sortStrings(keys)
	parts := make([]string, 0, len(sortedKeys))
	for _, k := range sortedKeys {
		if v := dims[k]; v != "" {
			parts = append(parts, k+":"+v)
		}
	}
	return strings.Join(parts, "|")
}

func sortStrings(ss []string) []string {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
	return ss
}

// repoToService extracts a short service name from "owner/repo".
func repoToService(fullName string) string {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fullName
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func verifySignature(body []byte, secret, sig string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}
