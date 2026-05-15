package artifactory_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
	_ "github.com/forge-eng-fabric/pkg/artifact-store-adapter/artifactory"
)

type artifactoryFake struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newFake() *artifactoryFake { return &artifactoryFake{objects: map[string][]byte{}} }

func (f *artifactoryFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/artifactory/api/system/ping" {
		_, _ = w.Write([]byte("OK"))
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/artifactory/") {
		w.WriteHeader(404)
		return
	}
	switch r.Method {
	case http.MethodHead:
		if _, ok := f.objects[r.URL.Path]; ok {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(404)
	case http.MethodPut:
		if _, ok := f.objects[r.URL.Path]; ok {
			w.WriteHeader(http.StatusConflict)
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.objects[r.URL.Path] = body
		w.WriteHeader(201)
	case http.MethodGet:
		b, ok := f.objects[r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write(b)
	case http.MethodDelete:
		delete(f.objects, r.URL.Path)
		w.WriteHeader(204)
	default:
		w.WriteHeader(405)
	}
}

type auditCapture struct {
	mu     sync.Mutex
	events []adapter.AuditEvent
}

func (c *auditCapture) EmitArtifactEvent(_ context.Context, e adapter.AuditEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return nil
}

func newDriver(t *testing.T, url string, tenant string) (adapter.Adapter, *auditCapture) {
	t.Helper()
	audit := &auditCapture{}
	f := adapter.NewFactory(adapter.StaticSecretFetcher{"vault://forge/test": []byte("admin:token")}, audit)
	cfg := adapter.BindingConfig{
		TenantID:      tenant,
		Backend:       adapter.BackendArtifactory,
		CredentialRef: "vault://forge/test",
		Settings: map[string]any{
			"base_url":    url,
			"repo_prefix": "forge-skills",
		},
	}
	a, err := f.Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return a, audit
}

func TestArtifactoryContract(t *testing.T) {
	fake := newFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	a, audit := newDriver(t, srv.URL, "t1")

	payload := []byte("artifactory skill bytes")
	digest := adapter.DigestSHA256(payload)
	obj := adapter.Object{TenantID: "t1", AssetID: "skill-art", Version: "1.0.0", Digest: digest}

	if err := a.Put(context.Background(), obj, strings.NewReader(string(payload)), int64(len(payload)), adapter.Manifest{Digest: digest}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := a.Get(context.Background(), obj)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("bytes mismatch")
	}
	if err := a.Put(context.Background(), obj, strings.NewReader(string(payload)), int64(len(payload)), adapter.Manifest{}); !adapter.IsCode(err, adapter.ErrCodeImmutable) {
		t.Fatalf("expected ErrCodeImmutable; got %v", err)
	}
	other := obj
	other.TenantID = "t2"
	if _, err := a.Get(context.Background(), other); !adapter.IsCode(err, adapter.ErrCodeCrossTenant) {
		t.Fatalf("expected ErrCodeCrossTenant; got %v", err)
	}

	audit.mu.Lock()
	gotPut, gotGet := 0, 0
	for _, e := range audit.events {
		if e.Op == "put" {
			gotPut++
		}
		if e.Op == "get" {
			gotGet++
		}
	}
	audit.mu.Unlock()
	if gotPut == 0 || gotGet == 0 {
		t.Fatalf("expected put + get audit events; got put=%d get=%d", gotPut, gotGet)
	}
}

func TestArtifactoryDigestMismatchOnPut(t *testing.T) {
	fake := newFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	a, _ := newDriver(t, srv.URL, "t1")
	obj := adapter.Object{TenantID: "t1", AssetID: "skill-bad", Version: "1.0.0", Digest: "sha256:" + strings.Repeat("0", 64)}
	err := a.Put(context.Background(), obj, strings.NewReader("hello"), 5, adapter.Manifest{})
	if !adapter.IsCode(err, adapter.ErrCodeDigestMismatch) {
		t.Fatalf("expected ErrCodeDigestMismatch; got %v", err)
	}
}
