// Package gateway is the CLI's HTTP client to the developer skill gateway.
package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client talks to a Forge skill gateway.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{BaseURL: baseURL, Token: token, HTTPClient: &http.Client{Timeout: 60 * time.Second}}
}

// Asset is the subset of fields the CLI needs from /v1/gateway/assets.
type Asset struct {
	ID            string         `json:"id"`
	Version       string         `json:"version"`
	Type          string         `json:"type"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	TrustLevel    string         `json:"trust_level"`
	PackageDigest *string        `json:"package_digest,omitempty"`
}

// List returns installable assets in the developer's workspace.
func (c *Client) List(ctx context.Context, query string) ([]Asset, error) {
	u := c.BaseURL + "/v1/gateway/assets"
	if query != "" {
		u += "?" + url.Values{"q": []string{query}}.Encode()
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	c.authHeader(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("gateway list: %s", resp.Status)
	}
	var out []Asset
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// Package downloads a packaged Agent Skills bundle. The returned digest is
// what the gateway advertised; the caller is responsible for verifying it
// matches sha256(bytes).
type Package struct {
	Bytes  []byte
	Digest string
}

// Download fetches and verifies the bundle.
func (c *Client) Download(ctx context.Context, assetID, version string) (*Package, error) {
	u := fmt.Sprintf("%s/v1/gateway/assets/%s/versions/%s/package", c.BaseURL, url.PathEscape(assetID), url.PathEscape(version))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	c.authHeader(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("gateway package: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	digest := resp.Header.Get("X-Forge-Package-Digest")
	if digest == "" {
		return nil, fmt.Errorf("gateway did not return X-Forge-Package-Digest")
	}
	computed := "sha256:" + hexSHA256(body)
	if computed != digest {
		return nil, fmt.Errorf("digest mismatch: gateway says %s, computed %s", digest, computed)
	}
	return &Package{Bytes: body, Digest: digest}, nil
}

func (c *Client) authHeader(r *http.Request) {
	if c.Token != "" {
		r.Header.Set("Authorization", "Bearer "+c.Token)
	}
}

func hexSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
