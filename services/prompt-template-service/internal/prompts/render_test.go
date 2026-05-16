package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderSubstitutesVariables(t *testing.T) {
	r := NewStubRenderer()
	resp, err := r.Render(context.Background(), RenderRequest{
		Ref:       "registry:prompt/sdlc-product/email-classify@1.3.0",
		Variables: map[string]any{"body": "outage now"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(resp.User, "outage now") {
		t.Errorf("placeholder not substituted: %q", resp.User)
	}
	if resp.TokenEstimate <= 0 {
		t.Errorf("token estimate should be > 0")
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	r := NewStubRenderer()
	_, err := r.Render(context.Background(), RenderRequest{
		Ref: "registry:prompt/nope/missing@1.0.0",
	})
	if !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("expected unknown_template, got %v", err)
	}
}

func TestRenderBadRef(t *testing.T) {
	r := NewStubRenderer()
	_, err := r.Render(context.Background(), RenderRequest{Ref: "not-a-ref"})
	if !errors.Is(err, ErrBadRef) {
		t.Fatalf("expected bad_template_ref, got %v", err)
	}
}

func TestRenderHTTPEndpoint(t *testing.T) {
	srv := &Server{Renderer: NewStubRenderer()}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	body := `{"ref":"registry:prompt/sdlc-product/email-classify@1.3.0","variables":{"body":"hello"}}`
	resp, err := http.Post(ts.URL+"/v1/render", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var out RenderResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if !strings.Contains(out.User, "hello") {
		t.Errorf("substitution missing in HTTP response: %q", out.User)
	}
}
