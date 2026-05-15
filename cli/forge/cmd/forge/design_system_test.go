package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDesignSystemList_Smoke(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/design-systems" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"name": "desing-system-1", "version": "1.0.0", "lifecycle_state": "approved", "visibility": "tenant_global"},
			{"name": "desing-system-2", "version": "1.0.0", "lifecycle_state": "approved", "visibility": "tenant_global"},
		})
	}))
	defer server.Close()
	t.Setenv("FORGE_REGISTRY_URL", server.URL)
	stdout := captureStdout(t, func() {
		cmd := designSystemCmd()
		cmd.SetArgs([]string{"list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("list: %v", err)
		}
	})
	if !strings.Contains(stdout, "desing-system-1") || !strings.Contains(stdout, "desing-system-2") {
		t.Fatalf("expected both built-ins in stdout, got %q", stdout)
	}
}

func TestDesignSystemSwap_PostsToApplication(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/design-system:swap") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"app":{"id":"a"},"swap_pr":{"pr_url":"https://example/pr/1"}}`))
	}))
	defer server.Close()
	t.Setenv("FORGE_APPLICATION_URL", server.URL)
	cmd := designSystemCmd()
	cmd.SetArgs([]string{"swap", "app-id-1", "--to", "design_system:platform:desing-system-3@2.0.0", "--reason", "Test"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("swap: %v", err)
	}
	if captured["target_ref"] != "design_system:platform:desing-system-3@2.0.0" {
		t.Fatalf("expected target_ref captured, got %+v", captured)
	}
	if captured["reason"] != "Test" {
		t.Fatalf("expected reason captured, got %+v", captured)
	}
}

func TestDesignSystemInstall_RequiresApp(t *testing.T) {
	cmd := designSystemCmd()
	cmd.SetArgs([]string{"install", "ds-forge-default"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--app is required") {
		t.Fatalf("expected --app required, got %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()
	done := make(chan string, 1)
	go func() {
		var buf []byte
		tmp := make([]byte, 4096)
		for {
			n, err := r.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if err != nil {
				break
			}
		}
		done <- string(buf)
	}()
	fn()
	_ = w.Close()
	return <-done
}
