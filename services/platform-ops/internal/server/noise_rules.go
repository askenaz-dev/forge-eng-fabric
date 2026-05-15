package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/forge-eng-fabric/services/platform-ops/internal/audit"
	"github.com/forge-eng-fabric/services/platform-ops/internal/github"
)

// GET /v1/noise-rules?status=draft
func (h *handler) noiseRuleList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "draft"
	}

	type noiseRuleItem struct {
		ID                string    `json:"id"`
		Fingerprint       string    `json:"fingerprint"`
		Description       string    `json:"description"`
		ProposedBy        string    `json:"proposed_by"`
		ProposedAt        time.Time `json:"proposed_at"`
		ApprovedBy        *string   `json:"approved_by,omitempty"`
		Status            string    `json:"status"`
		PrURL             *string   `json:"pr_url,omitempty"`
		EvidenceSampleIDs []string  `json:"evidence_sample_ids,omitempty"`
		ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	}

	var rules []noiseRuleItem
	if h.cfg.Pool != nil {
		rows, err := h.cfg.Pool.Query(r.Context(), `
			SELECT id, fingerprint, description, proposed_by, proposed_at,
			       approved_by, status, pr_url, evidence_sample_ids, expires_at
			FROM noise_rule
			WHERE status = $1
			ORDER BY proposed_at DESC
			LIMIT 100`, status)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db query failed")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var nr noiseRuleItem
			if err := rows.Scan(&nr.ID, &nr.Fingerprint, &nr.Description,
				&nr.ProposedBy, &nr.ProposedAt, &nr.ApprovedBy,
				&nr.Status, &nr.PrURL, &nr.EvidenceSampleIDs, &nr.ExpiresAt); err == nil {
				rules = append(rules, nr)
			}
		}
	}

	if rules == nil {
		rules = []noiseRuleItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rules":  rules,
		"status": status,
		"total":  len(rules),
	})
}

// POST /v1/noise-rules/propose
type NoiseRuleProposeRequest struct {
	Fingerprint       string   `json:"fingerprint"`
	Description       string   `json:"description"`
	EvidenceSampleIDs []string `json:"evidence_sample_ids,omitempty"`
	ExpiresAt         string   `json:"expires_at,omitempty"` // RFC3339
	SymptomID         string   `json:"symptom_id,omitempty"`
	SessionID         string   `json:"session_id,omitempty"`
}

func (h *handler) noiseRulePropose(w http.ResponseWriter, r *http.Request) {
	var body NoiseRuleProposeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Fingerprint == "" || body.Description == "" {
		writeError(w, http.StatusBadRequest, "fingerprint and description are required")
		return
	}

	actor := r.Header.Get("X-Forge-Actor")
	if actor == "" {
		actor = "unknown"
	}

	ruleID := uuid.NewString()
	now := time.Now().UTC()

	var expiresAt *time.Time
	if body.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, body.ExpiresAt); err == nil {
			expiresAt = &t
		}
	}

	// pgx v5 accepts []string for text[] columns; nil encodes as NULL.
	var evidence []string
	if len(body.EvidenceSampleIDs) > 0 {
		evidence = body.EvidenceSampleIDs
	}

	if h.cfg.Pool != nil {
		_, _ = h.cfg.Pool.Exec(r.Context(), `
			INSERT INTO noise_rule (id, fingerprint, description, proposed_by, proposed_at,
				evidence_sample_ids, expires_at, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft')`,
			ruleID, body.Fingerprint, body.Description, actor, now, evidence, expiresAt,
		)
	}

	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:   uuid.NewString(),
		Actor:     actor,
		Action:    "platform-ops:noise-rule:propose",
		Target:    body.Fingerprint,
		Outcome:   "success",
		SymptomID: body.SymptomID,
		SessionID: body.SessionID,
		Details: map[string]any{
			"rule_id":     ruleID,
			"description": body.Description,
		},
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"rule_id":     ruleID,
		"fingerprint": body.Fingerprint,
		"status":      "draft",
		"proposed_by": actor,
		"proposed_at": now,
	})
}

// POST /v1/noise-rules/{id}/approve
type NoiseRuleApproveRequest struct {
	SymptomID string `json:"symptom_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func (h *handler) noiseRuleApprove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body NoiseRuleApproveRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	actor := r.Header.Get("X-Forge-Actor")
	if actor == "" {
		actor = "unknown"
	}

	// Fetch rule from DB.
	type ruleRow struct {
		Fingerprint string
		Description string
	}
	var rule ruleRow
	if h.cfg.Pool != nil {
		row := h.cfg.Pool.QueryRow(r.Context(),
			`SELECT fingerprint, description FROM noise_rule WHERE id=$1 AND status='draft'`, id)
		if err := row.Scan(&rule.Fingerprint, &rule.Description); err != nil {
			writeError(w, http.StatusNotFound, "noise rule not found or not in draft status")
			return
		}
	} else {
		rule = ruleRow{Fingerprint: "unknown", Description: "fixture"}
	}

	now := time.Now().UTC()

	// Generate noise-rules.yaml content (canonical form of all active+this rule).
	yamlContent := buildNoiseRulesYAML(id, rule.Fingerprint, rule.Description, actor, now)

	// Create PR via GitHub App.
	branch := fmt.Sprintf("platform-ops/noise-rule-%s", id[:8])
	prTitle := fmt.Sprintf("chore(noise-rules): approve rule for %s", rule.Fingerprint)
	prBody := fmt.Sprintf("Automated PR from platform-ops.\n\n**Rule ID:** `%s`\n**Fingerprint:** `%s`\n**Description:** %s\n**Approved by:** %s",
		id, rule.Fingerprint, rule.Description, actor)

	gh := github.New(github.Config{
		Token: os.Getenv("GITHUB_TOKEN"),
		Owner: env("GITHUB_OWNER", "forge-eng-fabric"),
		Repo:  env("GITHUB_REPO", "forge-eng-fabric"),
	})

	pr, err := gh.CreateNoiseRulePR(r.Context(), branch, prTitle, prBody, yamlContent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create PR: "+err.Error())
		return
	}

	// Transactionally: mark rule active + record pr_url.
	if h.cfg.Pool != nil {
		_, _ = h.cfg.Pool.Exec(r.Context(),
			`UPDATE noise_rule SET status='active', approved_by=$1, approved_at=$2, pr_url=$3 WHERE id=$4`,
			actor, now, pr.HTMLURL, id,
		)
	}

	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:          actionID,
		Actor:            actor,
		Action:           "platform-ops:noise-rule:approve",
		Target:           rule.Fingerprint,
		Outcome:          "success",
		SymptomID:        body.SymptomID,
		SessionID:        body.SessionID,
		PolicyBundleHash: h.cfg.OPA.BundleHash(),
		Details: map[string]any{
			"rule_id": id,
			"pr_url":  pr.HTMLURL,
			"pr_num":  pr.Number,
		},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":    actionID,
		"rule_id":     id,
		"fingerprint": rule.Fingerprint,
		"status":      "active",
		"approved_by": actor,
		"approved_at": now,
		"pr_url":      pr.HTMLURL,
		"pr_number":   pr.Number,
	})
}

// POST /v1/noise-rules/{id}/revoke
type NoiseRuleRevokeRequest struct {
	Reason    string `json:"reason,omitempty"`
	SymptomID string `json:"symptom_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func (h *handler) noiseRuleRevoke(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body NoiseRuleRevokeRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	actor := r.Header.Get("X-Forge-Actor")
	if actor == "" {
		actor = "unknown"
	}

	// Fetch current rule.
	type ruleRow struct {
		Fingerprint string
		Description string
		PrURL       *string
	}
	var rule ruleRow
	if h.cfg.Pool != nil {
		row := h.cfg.Pool.QueryRow(r.Context(),
			`SELECT fingerprint, description, pr_url FROM noise_rule WHERE id=$1 AND status IN ('draft','active','promoted')`, id)
		if err := row.Scan(&rule.Fingerprint, &rule.Description, &rule.PrURL); err != nil {
			writeError(w, http.StatusNotFound, "noise rule not found or already revoked")
			return
		}
	} else {
		rule = ruleRow{Fingerprint: "unknown", Description: "fixture"}
	}

	now := time.Now().UTC()

	// Create revert PR (empty rules file or removal of this rule's entry).
	branch := fmt.Sprintf("platform-ops/noise-rule-revoke-%s", id[:8])
	prTitle := fmt.Sprintf("chore(noise-rules): revoke rule for %s", rule.Fingerprint)
	reason := body.Reason
	if reason == "" {
		reason = "manual revocation"
	}
	prBody := fmt.Sprintf("Automated revert PR from platform-ops.\n\n**Rule ID:** `%s`\n**Fingerprint:** `%s`\n**Reason:** %s\n**Revoked by:** %s",
		id, rule.Fingerprint, reason, actor)
	yamlContent := fmt.Sprintf("# Managed by platform-ops. Rule %s revoked at %s.\nrules: []\n", id, now.Format(time.RFC3339))

	gh := github.New(github.Config{
		Token: os.Getenv("GITHUB_TOKEN"),
		Owner: env("GITHUB_OWNER", "forge-eng-fabric"),
		Repo:  env("GITHUB_REPO", "forge-eng-fabric"),
	})

	pr, err := gh.CreateNoiseRulePR(r.Context(), branch, prTitle, prBody, yamlContent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create revert PR: "+err.Error())
		return
	}

	if h.cfg.Pool != nil {
		_, _ = h.cfg.Pool.Exec(r.Context(),
			`UPDATE noise_rule SET status='revoked', revoked_at=$1 WHERE id=$2`,
			now, id,
		)
	}

	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:   actionID,
		Actor:     actor,
		Action:    "platform-ops:noise-rule:revoke",
		Target:    rule.Fingerprint,
		Outcome:   "success",
		SymptomID: body.SymptomID,
		SessionID: body.SessionID,
		Details: map[string]any{
			"rule_id":      id,
			"reason":       reason,
			"revert_pr_url": pr.HTMLURL,
		},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":      actionID,
		"rule_id":       id,
		"fingerprint":   rule.Fingerprint,
		"status":        "revoked",
		"revoked_by":    actor,
		"revoked_at":    now,
		"revert_pr_url": pr.HTMLURL,
	})
}

// POST /v1/webhooks/github — GitHub App webhook; promotes noise rules on PR merge.
func (h *handler) githubWebhook(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || !verifyWebhookSig(body, secret, sig) {
			writeError(w, http.StatusUnauthorized, "invalid webhook signature")
			return
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
	}

	event := r.Header.Get("X-GitHub-Event")
	if event != "pull_request" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var payload struct {
		Action      string `json:"action"`
		PullRequest struct {
			Merged  bool   `json:"merged"`
			HTMLURL string `json:"html_url"`
		} `json:"pull_request"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if payload.Action != "closed" || !payload.PullRequest.Merged {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Promote all active rules whose pr_url matches the merged PR.
	if h.cfg.Pool != nil {
		now := time.Now().UTC()
		_, _ = h.cfg.Pool.Exec(r.Context(),
			`UPDATE noise_rule SET status='promoted', promoted_at=$1 WHERE pr_url=$2 AND status='active'`,
			now, payload.PullRequest.HTMLURL,
		)
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ----------------------------------------------------------------

func buildNoiseRulesYAML(ruleID, fingerprint, description, approvedBy string, approvedAt time.Time) string {
	return fmt.Sprintf(`# Managed by platform-ops. Do not edit manually.
# https://docs.forge-eng-fabric.internal/docs/alfred/noise-rules
rules:
  - id: %s
    fingerprint: %q
    description: %q
    approved_by: %q
    approved_at: %q
`, ruleID, fingerprint, description, approvedBy, approvedAt.Format(time.RFC3339))
}

func verifyWebhookSig(body []byte, secret, sig string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
