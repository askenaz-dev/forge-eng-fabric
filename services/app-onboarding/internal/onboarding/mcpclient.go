package onboarding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPGitHubMCP calls the GitHub MCP `/v1/invoke` endpoint.
type HTTPGitHubMCP struct {
	BaseURL string
	Client  *http.Client
}

func NewHTTPGitHubMCP(baseURL string) *HTTPGitHubMCP {
	return &HTTPGitHubMCP{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *HTTPGitHubMCP) invoke(ctx context.Context, req *Request, toolID string, params map[string]any) (map[string]any, error) {
	body := map[string]any{
		"tool_id": toolID,
		"params":  params,
		"context": map[string]any{
			"principal":      "service:app-onboarding",
			"workspace_id":   req.WorkspaceID,
			"correlation_id": req.CorrelationID,
		},
	}
	buf, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/invoke", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("X-Forge-Principal", "service:app-onboarding")
	httpReq.Header.Set("X-Correlation-Id", req.CorrelationID)
	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mcp %s failed: %d %s", toolID, resp.StatusCode, string(data))
	}
	var out struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("mcp %s decode: %w", toolID, err)
	}
	return out.Result, nil
}

func (c *HTTPGitHubMCP) repoFullName(req *Request) string {
	return req.RepoOrg + "/" + req.RepoName
}

func (c *HTTPGitHubMCP) CreateRepo(ctx context.Context, req *Request) (string, error) {
	res, err := c.invoke(ctx, req, "mcp:github.create_repo", map[string]any{"repo": c.repoFullName(req)})
	if err != nil {
		return "", err
	}
	url, _ := res["url"].(string)
	return url, nil
}

func (c *HTTPGitHubMCP) SetCodeowners(ctx context.Context, req *Request, content string) error {
	_, err := c.invoke(ctx, req, "mcp:github.set_codeowners", map[string]any{
		"repo": c.repoFullName(req), "content": content,
	})
	return err
}

func (c *HTTPGitHubMCP) AddPRTemplate(ctx context.Context, req *Request, content string) error {
	_, err := c.invoke(ctx, req, "mcp:github.add_pr_template", map[string]any{
		"repo": c.repoFullName(req), "template": content,
	})
	return err
}

func (c *HTTPGitHubMCP) SetBranchProtection(ctx context.Context, req *Request, rules map[string]any) error {
	_, err := c.invoke(ctx, req, "mcp:github.set_branch_protection", map[string]any{
		"repo": c.repoFullName(req), "branch": "main", "rules": rules,
	})
	return err
}

func (c *HTTPGitHubMCP) SetRequiredChecks(ctx context.Context, req *Request, checks []string) error {
	_, err := c.invoke(ctx, req, "mcp:github.set_required_checks", map[string]any{
		"repo": c.repoFullName(req), "checks": checks,
	})
	return err
}

// StubGitHubMCP is the in-memory test implementation. It records the calls
// it received so tests can assert on them.
type StubGitHubMCP struct {
	Calls []StubCall
}

type StubCall struct {
	Method string
	Repo   string
	Params map[string]any
}

func (s *StubGitHubMCP) record(method, repo string, params map[string]any) {
	s.Calls = append(s.Calls, StubCall{Method: method, Repo: repo, Params: params})
}

func (s *StubGitHubMCP) CreateRepo(_ context.Context, req *Request) (string, error) {
	full := req.RepoOrg + "/" + req.RepoName
	s.record("CreateRepo", full, nil)
	return "https://github.com/" + full, nil
}

func (s *StubGitHubMCP) SetCodeowners(_ context.Context, req *Request, content string) error {
	s.record("SetCodeowners", req.RepoOrg+"/"+req.RepoName, map[string]any{"content": content})
	return nil
}

func (s *StubGitHubMCP) AddPRTemplate(_ context.Context, req *Request, template string) error {
	s.record("AddPRTemplate", req.RepoOrg+"/"+req.RepoName, map[string]any{"template_bytes": len(template)})
	return nil
}

func (s *StubGitHubMCP) SetBranchProtection(_ context.Context, req *Request, rules map[string]any) error {
	s.record("SetBranchProtection", req.RepoOrg+"/"+req.RepoName, map[string]any{"rules": rules})
	return nil
}

func (s *StubGitHubMCP) SetRequiredChecks(_ context.Context, req *Request, checks []string) error {
	s.record("SetRequiredChecks", req.RepoOrg+"/"+req.RepoName, map[string]any{"checks": checks})
	return nil
}
