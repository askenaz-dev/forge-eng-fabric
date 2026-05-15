package main

// inlineHTTPClient is the runtime's wire-compatible implementation of
// runtime.MCPGatewayClient and runtime.A2AGatewayClient. It posts the
// JSON payload at `<gatewayURL><basePath>/{assetID}?tool={tool}` (MCP) or
// as a JSON-RPC envelope (A2A) and returns the response body + status.
//
// Both pkg/mcp-shim.Client.InvokeTool and pkg/a2a-shim.Client.Send have
// the same shape; this file exists so the workflow-runtime binary does
// not need to take a cross-module Go dependency on the shim packages.
// Production binaries that already pull the shim modules can replace
// these helpers with the shim implementations one-for-one.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type inlineHTTPClient struct {
	base     string
	prefix   string
	token    string
	http     *http.Client
	a2aMode  bool
}

func newInlineHTTPClient(base, prefix, token string) *inlineHTTPClient {
	return &inlineHTTPClient{
		base:    strings.TrimRight(base, "/"),
		prefix:  prefix,
		token:   token,
		http:    &http.Client{Timeout: 60 * time.Second},
		a2aMode: strings.HasSuffix(prefix, "/a2a"),
	}
}

// InvokeTool implements runtime.MCPGatewayClient.
func (c *inlineHTTPClient) InvokeTool(ctx context.Context, assetID, toolName string, body any) ([]byte, int, error) {
	if c.a2aMode {
		return nil, 0, fmt.Errorf("a2a client used as mcp client")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	u := fmt.Sprintf("%s%s/%s", c.base, c.prefix, url.PathEscape(assetID))
	if toolName != "" {
		u += "?tool=" + url.QueryEscape(toolName)
	}
	return c.post(ctx, u, payload)
}

// Send implements runtime.A2AGatewayClient.
func (c *inlineHTTPClient) Send(ctx context.Context, assetID, method string, params any) ([]byte, int, error) {
	if !c.a2aMode {
		return nil, 0, fmt.Errorf("mcp client used as a2a client")
	}
	env := map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("rpc-%d", time.Now().UnixNano()),
		"method":  method,
		"params":  params,
	}
	payload, _ := json.Marshal(env)
	u := fmt.Sprintf("%s%s/%s", c.base, c.prefix, url.PathEscape(assetID))
	return c.post(ctx, u, payload)
}

func (c *inlineHTTPClient) post(ctx context.Context, u string, payload []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("content-type", "application/json")
	if c.token != "" {
		req.Header.Set("authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}
