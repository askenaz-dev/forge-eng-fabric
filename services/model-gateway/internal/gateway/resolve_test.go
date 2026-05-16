package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveReturnsKnownModel(t *testing.T) {
	r := &StubResolver{}
	resp, err := r.Resolve(context.Background(), ResolveRequest{
		Ref: "gateway:model/claude-opus-4-7@latest-stable", WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resp.ModelID != "claude-opus-4-7" {
		t.Errorf("model_id: got %q", resp.ModelID)
	}
	if resp.Provider != "anthropic" {
		t.Errorf("provider: got %q", resp.Provider)
	}
	if !strings.Contains(resp.CredentialsRef, "ws-1") {
		t.Errorf("credentials_ref should include workspace_id: %q", resp.CredentialsRef)
	}
}

func TestResolveBadRef(t *testing.T) {
	r := &StubResolver{}
	_, err := r.Resolve(context.Background(), ResolveRequest{Ref: "not-a-ref"})
	if !errors.Is(err, ErrBadRef) {
		t.Fatalf("expected bad_model_ref, got %v", err)
	}
}

func TestResolveEnforcesWhitelist(t *testing.T) {
	r := &StubResolver{WS: StaticWhitelist{"ws-1": {"claude-haiku-4-5-20251001"}}}
	_, err := r.Resolve(context.Background(), ResolveRequest{
		Ref: "gateway:model/claude-opus-4-7@latest-stable", WorkspaceID: "ws-1",
	})
	if !errors.Is(err, ErrModelNotWhitelisted) {
		t.Fatalf("expected model_not_whitelisted, got %v", err)
	}
}

func TestResolveHTTPEndpoint(t *testing.T) {
	srv := &Server{Resolver: &StubResolver{}}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	body := `{"ref":"gateway:model/gpt-4o@latest","workspace_id":"ws-x"}`
	resp, err := http.Post(ts.URL+"/v1/resolve", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var out ResolveResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.ModelID != "gpt-4o" {
		t.Errorf("model_id: %q", out.ModelID)
	}
}
