package mcpshim

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// captureEmitter records every deprecation event the shim emits so the
// test can assert that exactly one fires per InvokeTool call.
type captureEmitter struct {
	mu     sync.Mutex
	events []DeprecationEvent
}

func (c *captureEmitter) Emit(_ context.Context, e DeprecationEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return nil
}

func (c *captureEmitter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func TestInvokeToolEmitsDeprecationEvent(t *testing.T) {
	// Fake "gateway" that simply echoes the request body so the shim has
	// something to return.
	var receivedAuth string
	var receivedTool string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("authorization")
		receivedTool = r.URL.Query().Get("tool")
		_ = json.NewEncoder(w).Encode(map[string]any{"echo": true})
	}))
	defer srv.Close()

	emit := &captureEmitter{}
	c := New(srv.URL, emit)
	c.IdentityToken = "alice.acme.ws1"

	body, status, err := c.InvokeTool(
		context.Background(),
		"vendor-x", "read_doc",
		map[string]any{"doc_id": "42"},
		WithCaller("workflow-runtime"),
		WithCorrelation("corr-9"),
	)
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	if status != 200 {
		t.Fatalf("status=%d", status)
	}
	if !strings.Contains(string(body), "echo") {
		t.Fatalf("body=%s", string(body))
	}
	if emit.count() != 1 {
		t.Fatalf("expected exactly 1 deprecation event; got %d", emit.count())
	}
	emit.mu.Lock()
	e := emit.events[0]
	emit.mu.Unlock()
	if e.AssetID != "vendor-x" || e.ToolName != "read_doc" || e.Caller != "workflow-runtime" || e.CorrelationID != "corr-9" {
		t.Fatalf("event=%+v", e)
	}
	if receivedAuth != "Bearer alice.acme.ws1" {
		t.Fatalf("auth header=%q", receivedAuth)
	}
	if receivedTool != "read_doc" {
		t.Fatalf("tool query=%q", receivedTool)
	}
}

func TestInvokeToolEmitsEventEvenOnGatewayError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = io.WriteString(w, `{"error":"upstream"}`)
	}))
	defer srv.Close()
	emit := &captureEmitter{}
	c := New(srv.URL, emit)
	_, status, err := c.InvokeTool(context.Background(), "x", "y", nil)
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	if status != 500 {
		t.Fatalf("status=%d", status)
	}
	if emit.count() != 1 {
		t.Fatalf("expected one event even on upstream error; got %d", emit.count())
	}
}

func TestEventTypeConstantStable(t *testing.T) {
	if EventTypeGatewayBypassDeprecated != "com.forge.runtime.gateway_bypass_deprecated.v1" {
		t.Fatalf("event type constant drift: %s", EventTypeGatewayBypassDeprecated)
	}
}
