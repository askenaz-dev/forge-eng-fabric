package codeartifact_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
	_ "github.com/forge-eng-fabric/pkg/artifact-store-adapter/codeartifact"
)

// caFake serves a minimal subset of the CodeArtifact REST API the
// driver exercises. The test does not verify the SigV4 signature byte-
// for-byte; instead it verifies the driver constructs valid request
// shapes and handles per-status mapping.
type caFake struct {
	mu     sync.Mutex
	assets map[string][]byte // keyed by asset query param
}

func newCAFake() *caFake { return &caFake{assets: map[string][]byte{}} }

func (f *caFake) key(r *http.Request) string {
	q := r.URL.Query()
	return q.Get("repository") + "/" + q.Get("package") + "/" + q.Get("packageVersion") + "/" + q.Get("asset")
}

func (f *caFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/v1/domain" && r.Method == http.MethodGet {
		_, _ = w.Write([]byte(`{"domain":"forge"}`))
		return
	}
	if r.URL.Path == "/v1/asset" {
		switch r.Method {
		case http.MethodPost:
			k := f.key(r)
			if _, exists := f.assets[k]; exists {
				w.WriteHeader(http.StatusConflict)
				return
			}
			b, _ := io.ReadAll(r.Body)
			f.assets[k] = b
			_ = json.NewEncoder(w).Encode(map[string]any{"asset": k})
		case http.MethodGet:
			k := f.key(r)
			b, ok := f.assets[k]
			if !ok {
				w.WriteHeader(404)
				return
			}
			_, _ = w.Write(b)
		default:
			w.WriteHeader(405)
		}
		return
	}
	if r.URL.Path == "/v1/package/versions/delete" {
		w.WriteHeader(204)
		return
	}
	w.WriteHeader(404)
}

type auditNoOp struct{}

func (auditNoOp) EmitArtifactEvent(context.Context, adapter.AuditEvent) error { return nil }

func newDriver(t *testing.T, url, tenant string) adapter.Adapter {
	t.Helper()
	creds := []byte(`{"access_key_id":"AKIATEST","secret_access_key":"secret","session_token":""}`)
	f := adapter.NewFactory(adapter.StaticSecretFetcher{"vault://forge/aws": creds}, auditNoOp{})
	a, err := f.Build(context.Background(), adapter.BindingConfig{
		TenantID:      tenant,
		Backend:       adapter.BackendCodeArtifact,
		CredentialRef: "vault://forge/aws",
		Settings: map[string]any{
			"region":   "us-east-1",
			"domain":   "forge",
			"endpoint": url,
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return a
}

func TestCodeArtifactContract(t *testing.T) {
	fake := newCAFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	a := newDriver(t, srv.URL, "t1")

	payload := []byte("codeartifact skill bytes")
	digest := adapter.DigestSHA256(payload)
	obj := adapter.Object{TenantID: "t1", AssetID: "skill-ca", Version: "1.0.0", Digest: digest}

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

	// Immutability: backend returns 409 on duplicate Post which the
	// driver maps to ErrCodeImmutable.
	if err := a.Put(context.Background(), obj, strings.NewReader(string(payload)), int64(len(payload)), adapter.Manifest{}); !adapter.IsCode(err, adapter.ErrCodeImmutable) {
		t.Fatalf("expected ErrCodeImmutable; got %v", err)
	}

	// Cross-tenant.
	other := obj
	other.TenantID = "t2"
	if _, err := a.Get(context.Background(), other); !adapter.IsCode(err, adapter.ErrCodeCrossTenant) {
		t.Fatalf("expected ErrCodeCrossTenant; got %v", err)
	}
}

func TestCodeArtifactDigestMismatchRejected(t *testing.T) {
	fake := newCAFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	a := newDriver(t, srv.URL, "t1")
	obj := adapter.Object{TenantID: "t1", AssetID: "bad", Version: "1.0.0", Digest: "sha256:" + strings.Repeat("0", 64)}
	err := a.Put(context.Background(), obj, strings.NewReader("hi"), 2, adapter.Manifest{})
	if !adapter.IsCode(err, adapter.ErrCodeDigestMismatch) {
		t.Fatalf("expected ErrCodeDigestMismatch; got %v", err)
	}
}

func TestCodeArtifactCredentialParseFromColonForm(t *testing.T) {
	fake := newCAFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	f := adapter.NewFactory(adapter.StaticSecretFetcher{"v://aws": []byte("AKIA:secret")}, auditNoOp{})
	if _, err := f.Build(context.Background(), adapter.BindingConfig{
		TenantID: "t1", Backend: adapter.BackendCodeArtifact, CredentialRef: "v://aws",
		Settings: map[string]any{"region": "us-east-1", "domain": "forge", "endpoint": srv.URL},
	}); err != nil {
		t.Fatalf("colon-form credential should be accepted: %v", err)
	}
}
