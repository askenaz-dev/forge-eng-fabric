package onboarding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPRegistryRegistrar struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewHTTPRegistryRegistrar(baseURL, token string) *HTTPRegistryRegistrar {
	return &HTTPRegistryRegistrar{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		Client:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (r *HTTPRegistryRegistrar) RegisterApplication(ctx context.Context, req *Request, repoURL string, metadata map[string]any) (string, error) {
	body := map[string]any{
		"type":            "application",
		"name":            req.RepoName,
		"description":     fmt.Sprintf("Application %s onboarded from %s@%s", req.RepoName, req.TemplateID, req.TemplateVersion),
		"version":         applicationVersion(req),
		"owner_team":      ownerTeam(req.Owners),
		"inputs_schema":   map[string]any{},
		"outputs_schema":  map[string]any{},
		"visibility":      "workspace",
		"lifecycle_state": "proposed",
		"trust_level":     "T1",
		"owners":          req.Owners,
		"metadata":        metadata,
	}
	buf, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/v1/workspaces/%s/assets", r.BaseURL, req.WorkspaceID), bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("X-Correlation-Id", req.CorrelationID)
	if r.Token != "" {
		httpReq.Header.Set("authorization", "Bearer "+r.Token)
	}
	resp, err := r.Client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("registry asset create %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("registry response missing asset id")
	}
	_ = repoURL
	return out.ID, nil
}

func applicationVersion(req *Request) string {
	if version, ok := req.Parameters["app_version"].(string); ok && version != "" {
		return version
	}
	return "0.1.0"
}

func ownerTeam(owners []string) string {
	owner := strings.TrimPrefix(firstOwner(owners), "@")
	owner = strings.ReplaceAll(owner, "/", "-")
	if owner == "" || owner == "unknown" {
		return "platform-engineering"
	}
	return owner
}
