// Package registry is the gateway's client to the internal asset registry.
// All reads happen here; the gateway is a strict read-side over the registry.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// Asset mirrors the registry's Asset JSON shape, restricted to the fields the
// gateway needs.
type Asset struct {
	ID             string         `json:"id"`
	Version        string         `json:"version"`
	Type           string         `json:"type"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	WorkspaceID    uuid.UUID      `json:"workspace_id"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	Visibility     string         `json:"visibility"`
	LifecycleState string         `json:"lifecycle_state"`
	TrustLevel     string         `json:"trust_level"`
	EvalScores     map[string]any `json:"eval_scores"`
	Distribution   Distribution   `json:"distribution"`
	Metadata       map[string]any `json:"metadata"`
}

// Distribution mirrors registry's Distribution block.
type Distribution struct {
	GatewayPublished   bool       `json:"gateway_published"`
	GatewayChannel     string     `json:"gateway_channel"`
	PackageDigest      *string    `json:"package_digest"`
	PackageSignedAt    *time.Time `json:"package_signed_at"`
	DeprecationPointer *string    `json:"deprecation_pointer"`
}

// Client talks to the internal registry over HTTP.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	// SystemToken is a service-to-service token. The gateway authenticates
	// against the registry as a privileged read-only service.
	SystemToken string
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:     baseURL,
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
		SystemToken: token,
	}
}

// ListWorkspaceAssets returns assets in a workspace, optionally filtered by
// type. The gateway uses this then post-filters by distribution.gateway_published.
func (c *Client) ListWorkspaceAssets(ctx context.Context, workspaceID uuid.UUID, assetType string) ([]Asset, error) {
	q := url.Values{}
	if assetType != "" {
		q.Set("type", assetType)
	}
	u := fmt.Sprintf("%s/v1/workspaces/%s/assets", c.BaseURL, workspaceID)
	if encoded := q.Encode(); encoded != "" {
		u += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if c.SystemToken != "" {
		req.Header.Set("authorization", "Bearer "+c.SystemToken)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("registry: %s", resp.Status)
	}
	var out []Asset
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAssetVersion fetches a single (asset, version) row.
func (c *Client) GetAssetVersion(ctx context.Context, assetID, version string) (*Asset, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/assets/%s/versions/%s", c.BaseURL, url.PathEscape(assetID), url.PathEscape(version)), nil)
	if err != nil {
		return nil, err
	}
	if c.SystemToken != "" {
		req.Header.Set("authorization", "Bearer "+c.SystemToken)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("registry: %s", resp.Status)
	}
	var a Asset
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

var ErrNotFound = errors.New("asset_not_found")
