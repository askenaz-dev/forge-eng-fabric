// Package mcpshim is the runtime compatibility shim that lets existing
// internal callers (workflow-runtime, runners, Alfred) reach an MCP
// through services/mcp-gateway without code changes beyond a single
// import swap. The shim forwards the call through the gateway and emits
// `com.forge.runtime.gateway_bypass_deprecated.v1` so observability can
// surface remaining direct-dial paths during the migration window.
//
// The shim is INTENTIONALLY narrow: it covers the request/response shape
// the runtime emits today, and nothing else. When tasks 6.1 (runner) and
// 6.2 (workflow-runtime) flip to the gateway, this shim is the seam they
// land on. Once `gateway.enforced=true` ships everywhere, this package
// disappears (task 10.4).
package mcpshim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the shim entry point. Construct one per runtime process at
// boot; reuse across calls. The zero value is unusable; use New().
type Client struct {
	gatewayURL string
	http       *http.Client
	emit       DeprecationEmitter
	// IdentityToken is the runtime's workload-identity token in the
	// `principal.tenant.workspace` form the gateway accepts in non-prod;
	// production wires SPIFFE/IRSA + JWT issuance here.
	IdentityToken string
}

// DeprecationEmitter is the seam over which the shim publishes the
// `gateway_bypass_deprecated.v1` event. Production wires this to the
// runtime's normal Kafka producer; tests use a recording stub.
type DeprecationEmitter interface {
	Emit(ctx context.Context, event DeprecationEvent) error
}

type DeprecationEvent struct {
	Subject       string    `json:"subject"`
	AssetID       string    `json:"asset_id"`
	ToolName      string    `json:"tool_name"`
	Caller        string    `json:"caller"`
	CorrelationID string    `json:"correlation_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func New(gatewayURL string, emit DeprecationEmitter) *Client {
	return &Client{
		gatewayURL: strings.TrimRight(gatewayURL, "/"),
		http:       &http.Client{Timeout: 60 * time.Second},
		emit:       emit,
	}
}

// InvokeTool is the single entry point. It POSTs to
// /v1/gw/mcp/{asset_id}?tool={tool_name} with the JSON request body and
// returns the response. The shim always emits the deprecation event
// before the call returns, even on error, so the migration runbook can
// observe usage even when the upstream MCP fails.
//
// Returns the raw response body — callers that need streaming (SSE) use
// InvokeToolStream.
func (c *Client) InvokeTool(ctx context.Context, assetID, toolName string, body any, opts ...CallOption) ([]byte, int, error) {
	if c.gatewayURL == "" {
		return nil, 0, errors.New("mcp-shim: gateway URL not configured")
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := c.buildRequest(ctx, assetID, toolName, bodyBytes, opts...)
	if err != nil {
		return nil, 0, err
	}
	c.emitDeprecation(ctx, assetID, toolName, callerFromOpts(opts), correlationFromOpts(opts))
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(resp.Body)
	return rb, resp.StatusCode, err
}

// InvokeToolStream returns the raw response body for callers that need
// SSE streaming. Caller is responsible for closing the returned reader;
// the deprecation event is emitted before the request is dispatched.
func (c *Client) InvokeToolStream(ctx context.Context, assetID, toolName string, body any, opts ...CallOption) (io.ReadCloser, int, error) {
	if c.gatewayURL == "" {
		return nil, 0, errors.New("mcp-shim: gateway URL not configured")
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := c.buildRequest(ctx, assetID, toolName, bodyBytes, opts...)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("accept", "text/event-stream, application/json")
	c.emitDeprecation(ctx, assetID, toolName, callerFromOpts(opts), correlationFromOpts(opts))
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.StatusCode, nil
}

func (c *Client) buildRequest(ctx context.Context, assetID, toolName string, bodyBytes []byte, opts ...CallOption) (*http.Request, error) {
	u, err := url.Parse(c.gatewayURL + "/v1/gw/mcp/" + url.PathEscape(assetID))
	if err != nil {
		return nil, err
	}
	if toolName != "" {
		q := u.Query()
		q.Set("tool", toolName)
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	if c.IdentityToken != "" {
		req.Header.Set("authorization", "Bearer "+c.IdentityToken)
	}
	cid := correlationFromOpts(opts)
	if cid != "" {
		req.Header.Set("X-Forge-Correlation-Id", cid)
	}
	return req, nil
}

func (c *Client) emitDeprecation(ctx context.Context, assetID, toolName, caller, correlationID string) {
	if c.emit == nil {
		return
	}
	_ = c.emit.Emit(ctx, DeprecationEvent{
		Subject:       fmt.Sprintf("asset/%s/tool/%s", assetID, toolName),
		AssetID:       assetID,
		ToolName:      toolName,
		Caller:        caller,
		CorrelationID: correlationID,
		OccurredAt:    time.Now().UTC(),
	})
}

// CallOption tweaks a single call. Use WithCaller to label the calling
// component (workflow-runtime, alfred-worker, etc.) and WithCorrelation
// to thread an upstream correlation id through.
type CallOption func(*callConfig)

type callConfig struct {
	caller        string
	correlationID string
}

func WithCaller(name string) CallOption          { return func(c *callConfig) { c.caller = name } }
func WithCorrelation(id string) CallOption       { return func(c *callConfig) { c.correlationID = id } }

func callerFromOpts(opts []CallOption) string {
	c := callConfig{}
	for _, o := range opts {
		o(&c)
	}
	return c.caller
}

func correlationFromOpts(opts []CallOption) string {
	c := callConfig{}
	for _, o := range opts {
		o(&c)
	}
	return c.correlationID
}

// EventTypeGatewayBypassDeprecated is the canonical CloudEvents type for
// the deprecation telemetry. Consumers subscribing to the events topic
// filter on this constant.
const EventTypeGatewayBypassDeprecated = "com.forge.runtime.gateway_bypass_deprecated.v1"
