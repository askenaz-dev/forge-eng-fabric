package nexus_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
	_ "github.com/forge-eng-fabric/pkg/artifact-store-adapter/nexus"
)

// nexusFake stands in for a Nexus raw repository. It keeps the per-path
// bytes in memory and rejects overwrites by responding 200 on HEAD and
// 400 on PUT for paths that already exist (matches Nexus's
// `writePolicy=ALLOW_ONCE`).
type nexusFake struct {
	mu      sync.Mutex
	objects map[string][]byte
	// corruptOnGet, when set to a path, mutates a single byte of the
	// stored value just before serving so VerifyingReadCloser can fail.
	corruptOnGet string
}

func newNexusFake() *nexusFake { return &nexusFake{objects: map[string][]byte{}} }

func (f *nexusFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if r.URL.Path == "/service/rest/v1/status" && r.Method == http.MethodGet {
		w.WriteHeader(200)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/repository/") {
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
			http.Error(w, "ALLOW_ONCE violation", 400)
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
		if f.corruptOnGet == r.URL.Path && len(b) > 0 {
			corrupted := make([]byte, len(b))
			copy(corrupted, b)
			corrupted[0] ^= 0xFF
			_, _ = w.Write(corrupted)
			return
		}
		_, _ = w.Write(b)
	case http.MethodDelete:
		if _, ok := f.objects[r.URL.Path]; !ok {
			w.WriteHeader(404)
			return
		}
		delete(f.objects, r.URL.Path)
		w.WriteHeader(204)
	default:
		w.WriteHeader(405)
	}
}

func newDriver(t *testing.T, fakeURL string, audit adapter.AuditSink, tenant string) adapter.Adapter {
	t.Helper()
	f := adapter.NewFactory(adapter.StaticSecretFetcher{"vault://forge/test": []byte("forge:test-pw")}, audit)
	cfg := adapter.BindingConfig{
		TenantID:      tenant,
		Backend:       adapter.BackendNexus,
		CredentialRef: "vault://forge/test",
		Settings: map[string]any{
			"base_url":    fakeURL,
			"repo_prefix": "forge-skills",
		},
	}
	a, err := f.Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return a
}

// auditCapture mirrors the harness in adapter/contract_test.go.
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

func (c *auditCapture) byOp(op string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, e := range c.events {
		if e.Op == op {
			n++
		}
	}
	return n
}

func TestNexusContract(t *testing.T) {
	fake := newNexusFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()

	audit := &auditCapture{}
	a := newDriver(t, srv.URL, audit, "t1")

	payload := []byte("forge skill bytes for the contract test")
	digest := adapter.DigestSHA256(payload)
	obj := adapter.Object{
		TenantID: "t1", AssetID: "skill-contract", Version: "1.0.0", Digest: digest,
	}

	// Put + Get round-trip.
	if err := a.Put(context.Background(), obj, byteReader(payload), int64(len(payload)), adapter.Manifest{
		AssetID: obj.AssetID, Version: obj.Version, Digest: digest, SizeBytes: int64(len(payload)),
	}); err != nil {
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

	// Immutability.
	if err := a.Put(context.Background(), obj, byteReader(payload), int64(len(payload)), adapter.Manifest{}); !adapter.IsCode(err, adapter.ErrCodeImmutable) {
		t.Fatalf("expected ErrCodeImmutable; got %v", err)
	}

	// Stat returns the manifest sidecar.
	m, err := a.Stat(context.Background(), obj)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if m.Digest != digest || m.AssetID != obj.AssetID {
		t.Fatalf("manifest mismatch: %+v", m)
	}

	// Cross-tenant guard.
	other := obj
	other.TenantID = "t2"
	if _, err := a.Get(context.Background(), other); !adapter.IsCode(err, adapter.ErrCodeCrossTenant) {
		t.Fatalf("expected ErrCodeCrossTenant; got %v", err)
	}

	// Audit events emitted for put + get.
	if audit.byOp("put") == 0 {
		t.Fatalf("expected put audit event")
	}
	if audit.byOp("get") == 0 {
		t.Fatalf("expected get audit event")
	}

	// Digest mismatch on Get when the backend returns corrupted bytes.
	corrobj := adapter.Object{TenantID: "t1", AssetID: "skill-corrupt", Version: "1.0.0", Digest: digest}
	if err := a.Put(context.Background(), corrobj, byteReader(payload), int64(len(payload)), adapter.Manifest{}); err != nil {
		t.Fatalf("Put corrobj: %v", err)
	}
	// Find the stored path: it's repository/forge-skills-t1/skill-corrupt/1.0.0/skill-corrupt-1.0.0.tar.zst
	fake.mu.Lock()
	for k := range fake.objects {
		if strings.HasSuffix(k, "skill-corrupt-1.0.0.tar.zst") {
			fake.corruptOnGet = k
			break
		}
	}
	fake.mu.Unlock()
	rc, err = a.Get(context.Background(), corrobj)
	if err != nil {
		t.Fatalf("Get corrupt: %v", err)
	}
	_, _ = io.ReadAll(rc)
	if err := rc.Close(); !adapter.IsCode(err, adapter.ErrCodeDigestMismatch) {
		t.Fatalf("expected ErrCodeDigestMismatch on Close; got %v", err)
	}
}

func TestNexusPutRejectsBadDigest(t *testing.T) {
	fake := newNexusFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	a := newDriver(t, srv.URL, &auditCapture{}, "t1")
	payload := []byte("hello")
	obj := adapter.Object{
		TenantID: "t1", AssetID: "skill-bad", Version: "1.0.0",
		Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	err := a.Put(context.Background(), obj, byteReader(payload), int64(len(payload)), adapter.Manifest{})
	if !adapter.IsCode(err, adapter.ErrCodeDigestMismatch) {
		t.Fatalf("expected ErrCodeDigestMismatch; got %v", err)
	}
}

func TestNexusDelete(t *testing.T) {
	fake := newNexusFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	a := newDriver(t, srv.URL, &auditCapture{}, "t1")
	payload := []byte("delete me")
	obj := adapter.Object{TenantID: "t1", AssetID: "skill-del", Version: "1.0.0", Digest: adapter.DigestSHA256(payload)}
	if err := a.Put(context.Background(), obj, byteReader(payload), int64(len(payload)), adapter.Manifest{}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := a.Delete(context.Background(), obj); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := a.Get(context.Background(), obj); !adapter.IsCode(err, adapter.ErrCodeNotFound) {
		t.Fatalf("expected ErrCodeNotFound after delete; got %v", err)
	}
}

func byteReader(b []byte) io.Reader { return strings.NewReader(string(b)) }
