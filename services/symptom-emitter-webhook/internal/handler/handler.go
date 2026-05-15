// Package handler receives webhooks from Linear, PagerDuty, and Slack
// and normalises them to symptom events on forge.symptoms.v1.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Producer publishes symptom events to Kafka.
type Producer interface {
	Publish(ctx context.Context, key string, value []byte) error
}

// Mux returns an http.ServeMux configured with all webhook endpoints.
func Mux(p Producer) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/webhooks/linear", linearHandler(p))
	mux.HandleFunc("/webhooks/pagerduty", pagerdutyHandler(p))
	mux.HandleFunc("/webhooks/slack", slackHandler(p))
	return mux
}

// ── Linear ────────────────────────────────────────────────────────────────────

func linearHandler(p Producer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		var payload struct {
			Action string `json:"action"`
			Type   string `json:"type"`
			Data   struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				State       struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"state"`
				Team struct {
					Key string `json:"key"`
				} `json:"team"`
				Labels []struct {
					Name string `json:"name"`
				} `json:"labels"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		// Emit only on issue creation or when an issue moves to a failure state.
		if payload.Type != "Issue" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		isFailure := payload.Data.State.Type == "started" || payload.Data.State.Name == "In Progress"
		hasAlertLabel := false
		for _, l := range payload.Data.Labels {
			if strings.Contains(strings.ToLower(l.Name), "alert") ||
				strings.Contains(strings.ToLower(l.Name), "incident") {
				hasAlertLabel = true
				break
			}
		}
		if payload.Action != "create" && !isFailure && !hasAlertLabel {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		service := payload.Data.Team.Key
		if service == "" {
			service = "unknown"
		}
		excerpt := payload.Data.Title
		if payload.Data.Description != "" && len(payload.Data.Description) < 200 {
			excerpt = payload.Data.Title + " — " + payload.Data.Description
		}
		emit(r.Context(), p, service, "linear-issue", "warning",
			fmt.Sprintf("linear:%s", payload.Data.ID), excerpt)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── PagerDuty ─────────────────────────────────────────────────────────────────

func pagerdutyHandler(p Producer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		var payload struct {
			Messages []struct {
				Event   string `json:"event"`
				Incident struct {
					ID       string `json:"id"`
					Title    string `json:"title"`
					Urgency  string `json:"urgency"`
					Service  struct {
						Name string `json:"name"`
					} `json:"service"`
					Body struct {
						Details string `json:"details"`
					} `json:"body"`
				} `json:"incident"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		for _, msg := range payload.Messages {
			if msg.Event != "incident.trigger" && msg.Event != "incident.escalate" {
				continue
			}
			severity := "warning"
			if msg.Incident.Urgency == "high" {
				severity = "critical"
			}
			service := msg.Incident.Service.Name
			if service == "" {
				service = "unknown"
			}
			excerpt := msg.Incident.Title
			if msg.Incident.Body.Details != "" {
				excerpt += " — " + truncate(msg.Incident.Body.Details, 200)
			}
			emit(r.Context(), p, service, "pagerduty-incident", severity,
				fmt.Sprintf("pagerduty:%s", msg.Incident.ID), excerpt)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── Slack ─────────────────────────────────────────────────────────────────────

func slackHandler(p Producer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		// Slack slash command or event API payload.
		var payload struct {
			Type  string `json:"type"`
			Event struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Channel string `json:"channel"`
				User    string `json:"user"`
			} `json:"event"`
			// Slash command fields
			Command string `json:"command"`
			Text    string `json:"text"`
			Channel string `json:"channel_name"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			// Try URL-encoded form (slash commands use application/x-www-form-urlencoded)
			if err := r.ParseForm(); err == nil {
				payload.Command = r.FormValue("command")
				payload.Text = r.FormValue("text")
				payload.Channel = r.FormValue("channel_name")
			}
		}

		// Only emit on /forge-alert mentions or app_mention events containing "alert".
		isAlertMention := strings.EqualFold(payload.Command, "/forge-alert") ||
			(payload.Event.Type == "app_mention" &&
				(strings.Contains(strings.ToLower(payload.Event.Text), "alert") ||
					strings.Contains(strings.ToLower(payload.Event.Text), "incident")))

		if !isAlertMention {
			// Respond with 200 OK for Slack URL verification challenge.
			if payload.Type == "url_verification" {
				var challenge struct {
					Challenge string `json:"challenge"`
				}
				_ = json.Unmarshal(body, &challenge)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"challenge": challenge.Challenge})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		text := payload.Text
		if text == "" {
			text = payload.Event.Text
		}
		channel := payload.Channel
		if channel == "" {
			channel = payload.Event.Channel
		}
		emit(r.Context(), p, "slack", "slack-alert-mention", "warning",
			fmt.Sprintf("slack:%s", channel), truncate(text, 500))
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── shared ────────────────────────────────────────────────────────────────────

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
}

func emit(ctx context.Context, p Producer, service, signal, severity, evidenceRef, excerpt string) {
	fingerprint := fmt.Sprintf("service:%s|signal:%s", service, signal)
	evt := symptomEvent{
		SymptomID:       uuid.NewString(),
		Fingerprint:     fingerprint,
		Signal:          signal,
		Service:         service,
		Severity:        severity,
		Emitter:         "symptom-emitter-webhook",
		ObservedAt:      time.Now().UTC().Format(time.RFC3339),
		SchemaVersion:   "v1",
		EvidenceExcerpt: truncate(excerpt, 1024),
	}
	b, err := json.Marshal(evt)
	if err != nil {
		slog.Error("marshal symptom event", "err", err)
		return
	}
	if err := p.Publish(ctx, fingerprint, b); err != nil {
		slog.Error("publish symptom event", "fingerprint", fingerprint, "err", err)
		return
	}
	slog.Info("symptom emitted",
		"source", evidenceRef,
		"signal", signal,
		"service", service,
		"symptom_id", evt.SymptomID,
	)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
