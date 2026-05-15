package a2ashim

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

func TestSendEmitsDeprecationEvent(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "ok"})
	}))
	defer srv.Close()
	emit := &captureEmitter{}
	c := New(srv.URL, emit)
	c.IdentityToken = "alice.acme.ws1"
	body, status, err := c.Send(context.Background(), "partner-a", "tasks/send",
		map[string]any{"task": map[string]any{"text": "hello"}},
		WithCaller("workflow-runtime"), WithCorrelation("c-1"),
	)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if status != 200 {
		t.Fatalf("status=%d", status)
	}
	if !strings.Contains(string(body), "result") {
		t.Fatalf("body=%s", string(body))
	}
	if emit.count() != 1 {
		t.Fatalf("expected 1 event; got %d", emit.count())
	}
	// Verify the wire body is JSON-RPC shaped.
	var parsed map[string]any
	_ = json.Unmarshal(receivedBody, &parsed)
	if parsed["jsonrpc"] != "2.0" || parsed["method"] != "tasks/send" {
		t.Fatalf("expected JSON-RPC envelope; got %v", parsed)
	}
}

func TestEventTypeConstantStable(t *testing.T) {
	if EventTypeGatewayBypassDeprecated != "com.forge.runtime.gateway_bypass_deprecated.v1" {
		t.Fatalf("event type drift: %s", EventTypeGatewayBypassDeprecated)
	}
}
