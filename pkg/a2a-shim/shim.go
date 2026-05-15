// Package a2ashim is the runtime compatibility shim that lets existing
// internal callers (workflow-runtime, alfred) talk to A2A agents through
// services/a2a-gateway. It forwards the JSON-RPC request through the
// gateway and emits `com.forge.runtime.gateway_bypass_deprecated.v1` so
// observability can track migration progress. The shim is removed once
// `gateway.enforced=true` ships globally (active-registry-gateways
// tasks.md 10.4).
package a2ashim

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

type Client struct {
	gatewayURL    string
	http          *http.Client
	emit          DeprecationEmitter
	IdentityToken string
}

type DeprecationEmitter interface {
	Emit(ctx context.Context, event DeprecationEvent) error
}

type DeprecationEvent struct {
	Subject       string    `json:"subject"`
	AssetID       string    `json:"asset_id"`
	Method        string    `json:"method"`
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

// JSONRPCRequest is the wire envelope the gateway expects.
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Send invokes tasks/send (or any other JSON-RPC method) against the
// agent identified by assetID, returning the response body and status.
func (c *Client) Send(ctx context.Context, assetID, method string, params any, opts ...CallOption) ([]byte, int, error) {
	if c.gatewayURL == "" {
		return nil, 0, errors.New("a2a-shim: gateway URL not configured")
	}
	body := JSONRPCRequest{JSONRPC: "2.0", ID: newID(), Method: method, Params: params}
	bb, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := c.buildRequest(ctx, assetID, bb, opts...)
	if err != nil {
		return nil, 0, err
	}
	c.emitDeprecation(ctx, assetID, method, callerFromOpts(opts), correlationFromOpts(opts))
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(resp.Body)
	return rb, resp.StatusCode, err
}

// Subscribe invokes tasks/sendSubscribe and returns the SSE reader. The
// caller is responsible for closing it; the deprecation event fires
// before the request is dispatched.
func (c *Client) Subscribe(ctx context.Context, assetID string, params any, opts ...CallOption) (io.ReadCloser, int, error) {
	if c.gatewayURL == "" {
		return nil, 0, errors.New("a2a-shim: gateway URL not configured")
	}
	body := JSONRPCRequest{JSONRPC: "2.0", ID: newID(), Method: "tasks/sendSubscribe", Params: params}
	bb, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := c.buildRequest(ctx, assetID, bb, opts...)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("accept", "text/event-stream, application/json")
	c.emitDeprecation(ctx, assetID, "tasks/sendSubscribe", callerFromOpts(opts), correlationFromOpts(opts))
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.StatusCode, nil
}

func (c *Client) buildRequest(ctx context.Context, assetID string, body []byte, opts ...CallOption) (*http.Request, error) {
	u, err := url.Parse(c.gatewayURL + "/v1/gw/a2a/" + url.PathEscape(assetID))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	if c.IdentityToken != "" {
		req.Header.Set("authorization", "Bearer "+c.IdentityToken)
	}
	if cid := correlationFromOpts(opts); cid != "" {
		req.Header.Set("X-Forge-Correlation-Id", cid)
	}
	return req, nil
}

func (c *Client) emitDeprecation(ctx context.Context, assetID, method, caller, correlationID string) {
	if c.emit == nil {
		return
	}
	_ = c.emit.Emit(ctx, DeprecationEvent{
		Subject: fmt.Sprintf("asset/%s/method/%s", assetID, method),
		AssetID: assetID, Method: method, Caller: caller,
		CorrelationID: correlationID, OccurredAt: time.Now().UTC(),
	})
}

type CallOption func(*callConfig)
type callConfig struct {
	caller        string
	correlationID string
}

func WithCaller(name string) CallOption    { return func(c *callConfig) { c.caller = name } }
func WithCorrelation(id string) CallOption { return func(c *callConfig) { c.correlationID = id } }

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

func newID() string { return fmt.Sprintf("rpc-%d", time.Now().UnixNano()) }

const EventTypeGatewayBypassDeprecated = "com.forge.runtime.gateway_bypass_deprecated.v1"
