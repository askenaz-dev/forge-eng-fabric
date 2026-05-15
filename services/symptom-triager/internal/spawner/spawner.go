// Package spawner is the triager's interface to alfred-agent-mode sessions.
// The triager is the ONLY caller of POST /v1/agent-mode/sessions with actor=system:alfred.
package spawner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SessionRequest is the payload for POST /v1/agent-mode/sessions.
type SessionRequest struct {
	// Core
	WorkspaceID    string `json:"workspace_id"`
	CorrelationID  string `json:"correlation_id"`
	ModelID        string `json:"model_id,omitempty"`
	AutononymyPreset string `json:"autonomy_preset"`

	// Non-human trigger fields (iter 2+)
	Actor          string `json:"actor"`
	TriggerSource  string `json:"trigger_source"`
	ActorSession   string `json:"actor_session,omitempty"`
	SymptomID      string `json:"symptom_id,omitempty"`
	PlaybookID     string `json:"playbook_id,omitempty"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
}

// SessionResponse is the abbreviated response from alfred.
type SessionResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

// Spawner creates alfred-agent-mode sessions on behalf of the triager.
type Spawner struct {
	alfredURL string
	token     string
	http      *http.Client
	enabled   bool
}

// New creates a Spawner. If enabled=false all calls return a no-op.
func New(alfredURL, token string, enabled bool) *Spawner {
	return &Spawner{
		alfredURL: alfredURL,
		token:     token,
		http:      &http.Client{Timeout: 15 * time.Second},
		enabled:   enabled,
	}
}

// Spawn creates a new agent-mode session for a symptom.
// Returns the session ID or an error.
func (s *Spawner) Spawn(ctx context.Context, req SessionRequest) (*SessionResponse, error) {
	if !s.enabled {
		return &SessionResponse{SessionID: "noop", Status: "disabled"}, nil
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("spawner: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		s.alfredURL+"/v1/agent-mode/sessions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("spawner: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.token)
	}
	// Mark this as a system:alfred trigger so alfred can enforce forbidden_trigger_source.
	httpReq.Header.Set("X-Forge-Actor", "system:alfred")
	httpReq.Header.Set("X-Forge-Trigger-Source", "symptom")

	resp, err := s.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("spawner: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spawner: alfred returned HTTP %d", resp.StatusCode)
	}

	var result SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("spawner: decode response: %w", err)
	}
	return &result, nil
}
