package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateHowTo(t *testing.T) {
	cases := []struct {
		name    string
		block   map[string]any
		wantErr string
	}{
		{
			name: "happy path",
			block: map[string]any{
				"install": map[string]any{"claude-code": "npx forge install foo"},
				"usage":   map[string]any{"typescript": "import { foo } from 'forge';"},
				"env":     []any{"FORGE_API_TOKEN"},
			},
		},
		{
			name:    "empty block",
			block:   map[string]any{},
			wantErr: "block is empty",
		},
		{
			name: "missing install",
			block: map[string]any{
				"usage": map[string]any{"go": "x := foo.New()"},
			},
			wantErr: "install: required",
		},
		{
			name: "empty install command",
			block: map[string]any{
				"install": map[string]any{"cli": "   "},
				"usage":   map[string]any{"go": "x := foo.New()"},
			},
			wantErr: "install.cli",
		},
		{
			name: "missing usage",
			block: map[string]any{
				"install": map[string]any{"cli": "forge install"},
			},
			wantErr: "usage: required",
		},
		{
			name: "env not array of strings",
			block: map[string]any{
				"install": map[string]any{"cli": "forge install"},
				"usage":   map[string]any{"go": "x := foo.New()"},
				"env":     []any{123},
			},
			wantErr: "env[0]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHowTo(tc.block)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateActiveSurfaceFamilies(t *testing.T) {
	cases := []struct {
		name    string
		block   map[string]any
		wantErr string
	}{
		{
			name:  "mcp with endpoint path",
			block: map[string]any{"family": "mcp", "endpoint": "/v1/gw/mcp/foo"},
		},
		{
			name:  "a2a with absolute url",
			block: map[string]any{"family": "a2a", "endpoint": "https://gw.forge.example.com/v1/gw/a2a/foo"},
		},
		{
			name: "skill with artifact pointer",
			block: map[string]any{
				"family":           "skill",
				"artifact_pointer": "nexus://forge-skills/foo@1.2.3",
				"digest":           "sha256:" + strings.Repeat("a", 64),
				"signature_id":     "cosign://forge-skills/foo@sha256:abc",
			},
		},
		{
			name:    "unknown family",
			block:   map[string]any{"family": "unicorn"},
			wantErr: "must be one of mcp, a2a, skill",
		},
		{
			name:    "mcp missing endpoint",
			block:   map[string]any{"family": "mcp"},
			wantErr: "endpoint: required",
		},
		{
			name: "skill missing artifact pointer scheme",
			block: map[string]any{
				"family":           "skill",
				"artifact_pointer": "/no/scheme",
				"digest":           "sha256:" + strings.Repeat("a", 64),
				"signature_id":     "cosign://x",
			},
			wantErr: "adapter scheme",
		},
		{
			name: "skill digest not sha256",
			block: map[string]any{
				"family":           "skill",
				"artifact_pointer": "nexus://forge-skills/foo@1.2.3",
				"digest":           "md5:abc",
				"signature_id":     "cosign://x",
			},
			wantErr: "must be sha256",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateActiveSurface(tc.block)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateAssetActiveSurfaceTypeMismatch(t *testing.T) {
	howTo := map[string]any{
		"install": map[string]any{"cli": "forge install"},
		"usage":   map[string]any{"go": "x := foo.New()"},
	}
	surface := map[string]any{
		"family": "mcp", "endpoint": "/v1/gw/mcp/foo",
	}
	if fault := validateAssetActiveSurface("agent", howTo, surface); fault == nil || fault.Code != "active_surface_type_mismatch" {
		t.Fatalf("expected active_surface_type_mismatch for agent asset with family=mcp, got %#v", fault)
	}
	if fault := validateAssetActiveSurface("mcp", howTo, surface); fault != nil {
		t.Fatalf("expected mcp asset with family=mcp to pass, got %v", fault)
	}
	if fault := validateAssetActiveSurface("workflow", howTo, surface); fault != nil {
		t.Fatalf("workflow has no expected family, should pass; got %v", fault)
	}
}

func TestValidateAssetActiveSurfaceMissing(t *testing.T) {
	if fault := validateAssetActiveSurface("mcp", nil, map[string]any{"family": "mcp", "endpoint": "/v1/gw/mcp/x"}); fault == nil || fault.Code != "missing_how_to" {
		t.Fatalf("expected missing_how_to, got %#v", fault)
	}
	howTo := map[string]any{
		"install": map[string]any{"cli": "forge install"},
		"usage":   map[string]any{"go": "x := foo.New()"},
	}
	if fault := validateAssetActiveSurface("mcp", howTo, nil); fault == nil || fault.Code != "missing_active_surface" {
		t.Fatalf("expected missing_active_surface, got %#v", fault)
	}
}

func TestStatusForValidationFault(t *testing.T) {
	for _, code := range []string{"missing_how_to", "missing_active_surface", "drift_detected"} {
		if got := statusForValidationFault(code); got != http.StatusConflict {
			t.Fatalf("%s should map to 409, got %d", code, got)
		}
	}
	for _, code := range []string{"invalid_how_to", "invalid_active_surface", "active_surface_type_mismatch", "anything-else"} {
		if got := statusForValidationFault(code); got != http.StatusBadRequest {
			t.Fatalf("%s should map to 400, got %d", code, got)
		}
	}
}

func TestJoinAgentCardURL(t *testing.T) {
	cases := map[string]string{
		"https://x.example.com":          "https://x.example.com/.well-known/agent.json",
		"https://x.example.com/":         "https://x.example.com/.well-known/agent.json",
		"https://x.example.com/agents/a": "https://x.example.com/agents/a/.well-known/agent.json",
	}
	for in, want := range cases {
		got, err := joinAgentCardURL(in)
		if err != nil {
			t.Fatalf("joinAgentCardURL(%q) errored: %v", in, err)
		}
		if got != want {
			t.Fatalf("joinAgentCardURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultFetcherHashesStableBytes(t *testing.T) {
	body := []byte(`{"tools":[{"name":"echo"}]}`)
	want := "sha256:" + hex.EncodeToString(func() []byte { s := sha256.Sum256(body); return s[:] }())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	f := newDefaultFetcher()
	got, err := f.FetchManifest(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	if got != want {
		t.Fatalf("hash mismatch: got %s want %s", got, want)
	}
}

func TestDefaultFetcherSurfacesNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", 503)
	}))
	defer srv.Close()
	f := newDefaultFetcher()
	if _, err := f.FetchManifest(t.Context(), srv.URL); err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("expected status 503 error, got %v", err)
	}
}

func TestCredentialRefPattern(t *testing.T) {
	good := []string{
		"vault://kv/forge/foo",
		"aws-sm://us-west-2/forge-foo",
		"gcp-sm://projects/123/secrets/forge-foo/versions/latest",
		"azure-kv://forge-kv.vault.azure.net/secrets/forge-foo",
	}
	for _, ref := range good {
		if !credentialRefPattern.MatchString(ref) {
			t.Fatalf("expected %q to match credentialRefPattern", ref)
		}
	}
	for _, ref := range []string{"https://leaky.example.com/secret", "raw-secret", ""} {
		if credentialRefPattern.MatchString(ref) {
			t.Fatalf("expected %q to NOT match credentialRefPattern", ref)
		}
	}
}
