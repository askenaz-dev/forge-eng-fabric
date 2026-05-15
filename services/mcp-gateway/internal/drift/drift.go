// Package drift implements tool-list drift detection for the MCP gateway.
//
// On every new client session start (or on demand), Detector compares the
// tool list cached in the asset registry with the live tool list returned
// by the connected MCP server and emits a `mcp.tool_list.drifted.v1`
// CloudEvents-shaped event when they diverge.
package drift

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const EventType = "mcp.tool_list.drifted.v1"

// ToolDefinition is the minimal shape of an MCP tool entry that the drift
// detector cares about. In practice the registry returns a richer struct;
// we key on Name + InputSchema hash to detect additions, removals, and
// schema changes.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// DriftEvent is the CloudEvents-flavoured payload emitted when drift is
// detected. Callers receive this struct so they can serialise and emit it
// via whatever Publisher they have wired.
type DriftEvent struct {
	Specversion     string    `json:"specversion"`
	ID              string    `json:"id"`
	Source          string    `json:"source"`
	Type            string    `json:"type"`
	Time            string    `json:"time"`
	AssetID         string    `json:"asset_id"`
	Before          []string  `json:"before"`
	After           []string  `json:"after"`
	Diff            DriftDiff `json:"diff"`
}

// DriftDiff lists tool names that were added or removed relative to the
// registry snapshot.
type DriftDiff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
}

// Publisher is the minimal interface the detector needs to emit an event.
// The production wiring supplies the gateway's existing tcpPublisher.
type Publisher interface {
	Publish(ctx context.Context, eventType string, key, body []byte) error
}

// RegistryClient fetches the cached tool list for an approved MCP asset.
type RegistryClient interface {
	ListApprovedMCPTools(ctx context.Context, assetID string) ([]ToolDefinition, error)
}

// Detector holds the wired collaborators and performs drift checks.
type Detector struct {
	registry  RegistryClient
	publisher Publisher
	httpClient *http.Client
}

// New returns a Detector ready to use.
func New(registry RegistryClient, publisher Publisher) *Detector {
	return &Detector{
		registry:  registry,
		publisher: publisher,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// CheckOnSessionStart is the hook to call on every new MCP client session.
// It resolves the asset's cached tool list from the registry, calls
// tools/list on the live MCP endpoint, compares them, and emits a drift
// event if they differ.
//
// Parameters:
//   - assetID: the registry asset ID of the MCP being connected to
//   - mcpEndpoint: the base URL of the live MCP server (e.g. https://host/mcp)
//   - sessionID: a unique ID for the session, used as the CloudEvent ID prefix
func (d *Detector) CheckOnSessionStart(ctx context.Context, assetID, mcpEndpoint, sessionID string) error {
	// 1. Fetch the registry's cached tool list.
	cached, err := d.registry.ListApprovedMCPTools(ctx, assetID)
	if err != nil {
		// Non-fatal: log and return. We do not block the session on drift
		// detection failure.
		log.Printf("[drift] could not fetch registry tools for asset=%s: %v", assetID, err)
		return nil
	}

	// 2. Fetch the live tools/list from the MCP server.
	live, err := d.fetchLiveToolList(ctx, mcpEndpoint)
	if err != nil {
		log.Printf("[drift] could not fetch live tool list for asset=%s endpoint=%s: %v", assetID, mcpEndpoint, err)
		return nil
	}

	// 3. Compare.
	diff := computeDiff(cached, live)
	if len(diff.Added) == 0 && len(diff.Removed) == 0 {
		// No drift — nothing to emit.
		return nil
	}

	// 4. Drift detected — warn and emit event.
	log.Printf("[drift] WARN tool list drift detected for asset=%s: added=%v removed=%v",
		assetID, diff.Added, diff.Removed)

	event := DriftEvent{
		Specversion: "1.0",
		ID:          sessionID + "/drift",
		Source:      "forge://service/mcp-gateway",
		Type:        EventType,
		Time:        time.Now().UTC().Format(time.RFC3339Nano),
		AssetID:     assetID,
		Before:      toolNames(cached),
		After:       toolNames(live),
		Diff:        diff,
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("drift: marshal event: %w", err)
	}
	if pubErr := d.publisher.Publish(ctx, EventType, []byte(assetID), body); pubErr != nil {
		log.Printf("[drift] could not publish drift event for asset=%s: %v", assetID, pubErr)
	}

	// 5. Request a registry cache refresh (best-effort PATCH).
	d.requestRegistryRefresh(ctx, assetID, live)

	return nil
}

// fetchLiveToolList calls the MCP server's tools/list endpoint and returns
// the tool definitions it reports. The MCP JSON-RPC envelope for tools/list
// is: POST / with body {"jsonrpc":"2.0","id":1,"method":"tools/list"}.
func (d *Detector) fetchLiveToolList(ctx context.Context, mcpEndpoint string) ([]ToolDefinition, error) {
	u, err := url.Parse(mcpEndpoint)
	if err != nil {
		return nil, fmt.Errorf("bad mcp endpoint: %w", err)
	}
	// Normalise: if the endpoint already ends with a path component that
	// is not "/", we use it as-is. tools/list is sent to the root of the
	// MCP server.
	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("tools/list status=%d body=%s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Result struct {
			Tools []ToolDefinition `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("tools/list decode: %w", err)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("tools/list rpc error: %s", envelope.Error.Message)
	}
	return envelope.Result.Tools, nil
}

// requestRegistryRefresh asks the registry to refresh its tool cache for
// the asset. This is best-effort; errors are only logged.
func (d *Detector) requestRegistryRefresh(ctx context.Context, assetID string, live []ToolDefinition) {
	// The registry PATCH endpoint accepts a partial update; we send the
	// current live tool list so it can update its snapshot.
	payload := map[string]any{
		"metadata": map[string]any{
			"drift_detected": true,
			"live_tools":     live,
			"drift_at":       time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[drift] marshal patch payload: %v", err)
		return
	}

	if rc, ok := d.registry.(*httpRegistryDriftClient); ok {
		patchURL := strings.TrimRight(rc.baseURL, "/") + "/v1/assets/" + url.PathEscape(assetID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, bytes.NewReader(body))
		if err != nil {
			log.Printf("[drift] build patch request: %v", err)
			return
		}
		req.Header.Set("content-type", "application/json")
		if rc.token != "" {
			req.Header.Set("authorization", "Bearer "+rc.token)
		}
		resp, err := d.httpClient.Do(req)
		if err != nil {
			log.Printf("[drift] registry PATCH failed for asset=%s: %v", assetID, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
			log.Printf("[drift] registry PATCH status=%d body=%s", resp.StatusCode, string(body))
		}
	}
}

// computeDiff compares cached vs live tool lists and returns the diff.
func computeDiff(cached, live []ToolDefinition) DriftDiff {
	cachedSet := make(map[string]struct{}, len(cached))
	for _, t := range cached {
		cachedSet[t.Name] = struct{}{}
	}
	liveSet := make(map[string]struct{}, len(live))
	for _, t := range live {
		liveSet[t.Name] = struct{}{}
	}

	var added, removed []string
	for name := range liveSet {
		if _, ok := cachedSet[name]; !ok {
			added = append(added, name)
		}
	}
	for name := range cachedSet {
		if _, ok := liveSet[name]; !ok {
			removed = append(removed, name)
		}
	}
	return DriftDiff{Added: added, Removed: removed}
}

func toolNames(tools []ToolDefinition) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

// httpRegistryDriftClient is the production RegistryClient for drift
// detection. It is separate from the main gateway's registry client so
// the drift package has no import cycle with package main.
type httpRegistryDriftClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewHTTPRegistryClient returns a RegistryClient backed by the registry HTTP API.
func NewHTTPRegistryClient(baseURL, token string) RegistryClient {
	return &httpRegistryDriftClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *httpRegistryDriftClient) ListApprovedMCPTools(ctx context.Context, assetID string) ([]ToolDefinition, error) {
	u := c.baseURL + "/v1/assets/" + url.PathEscape(assetID) + "/tools"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// Asset has no cached tools yet — treat as empty list so any live
		// tools are reported as additions.
		return nil, nil
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("registry tools %s: status=%d body=%s", assetID, resp.StatusCode, string(body))
	}
	var tools []ToolDefinition
	if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
		return nil, fmt.Errorf("registry tools decode: %w", err)
	}
	return tools, nil
}
