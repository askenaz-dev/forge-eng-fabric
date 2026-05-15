// Package github provides a minimal GitHub API client for noise-rule PR lifecycle.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls the GitHub REST API using an installation token.
type Client struct {
	token  string
	apiURL string
	owner  string
	repo   string
	http   *http.Client
}

// Config holds Client construction parameters.
type Config struct {
	Token  string // installation or PAT token; empty = fixture mode
	APIURL string // defaults to https://api.github.com
	Owner  string
	Repo   string
}

// New creates a Client. If Token is empty the client operates in fixture mode
// (returns stub responses so CI and local dev work without GitHub credentials).
func New(cfg Config) *Client {
	apiURL := strings.TrimRight(cfg.APIURL, "/")
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	return &Client{
		token:  strings.TrimSpace(cfg.Token),
		apiURL: apiURL,
		owner:  cfg.Owner,
		repo:   cfg.Repo,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

// PR is the subset of GitHub PR fields needed by platform-ops.
type PR struct {
	HTMLURL string `json:"html_url"`
	Number  int    `json:"number"`
}

// FilePatch describes a single file to commit.
type FilePatch struct {
	Path    string
	Content string
}

// CodeFixPRInput is the input for OpenCodeFixPR.
type CodeFixPRInput struct {
	Branch        string
	CommitMessage string
	PRTitle       string
	PRBody        string
	Files         []FilePatch
}

// OpenCodeFixPR creates a branch, commits all files in one shot, and opens a PR.
// It NEVER merges the PR.
func (c *Client) OpenCodeFixPR(ctx context.Context, input CodeFixPRInput) (*PR, error) {
	if c.token == "" {
		return &PR{
			HTMLURL: fmt.Sprintf("https://github.com/%s/%s/pull/0", c.owner, c.repo),
			Number:  0,
		}, nil
	}

	mainSHA, err := c.getRefSHA(ctx, "heads/main")
	if err != nil {
		return nil, fmt.Errorf("get main sha: %w", err)
	}
	if err := c.createRef(ctx, "refs/heads/"+input.Branch, mainSHA); err != nil {
		return nil, fmt.Errorf("create branch %s: %w", input.Branch, err)
	}

	for _, f := range input.Files {
		fileSHA, _ := c.getFileSHA(ctx, f.Path, "main")
		if err := c.upsertFile(ctx, f.Path, input.Branch, input.CommitMessage, f.Content, fileSHA); err != nil {
			return nil, fmt.Errorf("upsert %s: %w", f.Path, err)
		}
	}

	pr, err := c.createPR(ctx, input.PRTitle, input.PRBody, input.Branch, "main")
	if err != nil {
		return nil, fmt.Errorf("create pr: %w", err)
	}
	return pr, nil
}

// CreateNoiseRulePR creates a branch, upserts policies/noise-rules.yaml with
// content, and opens a pull request against main.
func (c *Client) CreateNoiseRulePR(ctx context.Context, branch, title, prBody, content string) (*PR, error) {
	if c.token == "" {
		return &PR{
			HTMLURL: fmt.Sprintf("https://github.com/%s/%s/pull/0", c.owner, c.repo),
			Number:  0,
		}, nil
	}

	mainSHA, err := c.getRefSHA(ctx, "heads/main")
	if err != nil {
		return nil, fmt.Errorf("get main sha: %w", err)
	}
	if err := c.createRef(ctx, "refs/heads/"+branch, mainSHA); err != nil {
		return nil, fmt.Errorf("create branch %s: %w", branch, err)
	}

	filePath := "policies/noise-rules.yaml"
	fileSHA, _ := c.getFileSHA(ctx, filePath, "main")
	if err := c.upsertFile(ctx, filePath, branch, "chore: update noise rules", content, fileSHA); err != nil {
		return nil, fmt.Errorf("upsert %s: %w", filePath, err)
	}

	pr, err := c.createPR(ctx, title, prBody, branch, "main")
	if err != nil {
		return nil, fmt.Errorf("create pr: %w", err)
	}
	return pr, nil
}

// ---- internal helpers -------------------------------------------------------

func (c *Client) getRefSHA(ctx context.Context, ref string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/ref/%s", c.apiURL, c.owner, c.repo, ref)
	var out struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &out); err != nil {
		return "", err
	}
	return out.Object.SHA, nil
}

func (c *Client) createRef(ctx context.Context, ref, sha string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs", c.apiURL, c.owner, c.repo)
	body := map[string]string{"ref": ref, "sha": sha}
	return c.doJSON(ctx, http.MethodPost, url, body, nil)
}

func (c *Client) getFileSHA(ctx context.Context, path, ref string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", c.apiURL, c.owner, c.repo, path, ref)
	var out struct {
		SHA string `json:"sha"`
	}
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &out); err != nil {
		return "", err
	}
	return out.SHA, nil
}

func (c *Client) upsertFile(ctx context.Context, path, branch, message, content, existingSHA string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.apiURL, c.owner, c.repo, path)
	body := map[string]any{
		"message": message,
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"branch":  branch,
	}
	if existingSHA != "" {
		body["sha"] = existingSHA
	}
	return c.doJSON(ctx, http.MethodPut, url, body, nil)
}

func (c *Client) createPR(ctx context.Context, title, body, head, base string) (*PR, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", c.apiURL, c.owner, c.repo)
	reqBody := map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	}
	var pr PR
	if err := c.doJSON(ctx, http.MethodPost, url, reqBody, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

func (c *Client) doJSON(ctx context.Context, method, url string, reqBody, respBody any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("github %s %s: %d %s", method, url, resp.StatusCode, string(raw))
	}
	if respBody != nil && len(raw) > 0 {
		return json.Unmarshal(raw, respBody)
	}
	return nil
}
