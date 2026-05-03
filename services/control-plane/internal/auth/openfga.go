package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenFGAClient is a tiny REST client. We avoid the official SDK to keep
// dependencies minimal; the surface we need is just /check and /write.
type OpenFGAClient struct {
	URL     string
	StoreID string
	ModelID string
	HTTP    *http.Client
}

func NewOpenFGAClient(url, store, model string) (*OpenFGAClient, error) {
	if url == "" {
		return nil, errors.New("openfga url required")
	}
	return &OpenFGAClient{URL: url, StoreID: store, ModelID: model, HTTP: &http.Client{Timeout: 5 * time.Second}}, nil
}

// Check evaluates whether `user` has `relation` on `object`.
// `user` should be e.g. "user:alice". `object` e.g. "workspace:<uuid>".
func (c *OpenFGAClient) Check(ctx context.Context, user, relation, object string) (bool, error) {
	if c.StoreID == "" {
		// In Phase 0 dev we allow this to be unset and default-allow.
		return true, nil
	}
	body, _ := json.Marshal(map[string]any{
		"authorization_model_id": c.ModelID,
		"tuple_key": map[string]string{
			"user":     user,
			"relation": relation,
			"object":   object,
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/stores/%s/check", c.URL, c.StoreID), bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return false, fmt.Errorf("openfga check %d: %s", resp.StatusCode, string(data))
	}
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return false, err
	}
	return out.Allowed, nil
}

// Write writes a single tuple.
func (c *OpenFGAClient) Write(ctx context.Context, user, relation, object string) error {
	if c.StoreID == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]any{
		"authorization_model_id": c.ModelID,
		"writes": map[string]any{
			"tuple_keys": []map[string]string{
				{"user": user, "relation": relation, "object": object},
			},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/stores/%s/write", c.URL, c.StoreID), bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openfga write %d: %s", resp.StatusCode, string(data))
	}
	return nil
}
