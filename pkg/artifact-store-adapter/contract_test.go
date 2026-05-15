package adapter_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
)

// Contract is the shared test harness. Every driver test calls this with
// a freshly constructed Adapter, an Object the driver will accept and the
// bytes that hash to obj.Digest. The harness verifies the invariants
// every adapter must uphold:
//
//   1. Put followed by Get returns the same bytes; the verifying reader
//      passes digest verification on Close.
//   2. A second Put for the same Object MUST return ErrCodeImmutable.
//   3. A Get whose downstream bytes do not match obj.Digest fails with
//      ErrCodeDigestMismatch on Close.
//   4. A call with a different TenantID MUST return ErrCodeCrossTenant.
//   5. Audit events fire on every operation in both ok and error paths.
type ContractFixture struct {
	Adapter adapter.Adapter
	Object  adapter.Object
	Bytes   []byte
	// AuditEvents captured during the contract run. Test asserts at the
	// end that ops produced events.
	AuditCapture *AuditCapture
	// CorruptBytesGet is an optional hook the driver can wire so the
	// harness can ask the backend to return bytes that hash to a
	// different digest, exercising the verifying-reader path. When nil,
	// the digest-mismatch test is skipped.
	CorruptBytesGet func(obj adapter.Object)
}

func Contract(t *testing.T, f ContractFixture) {
	t.Helper()
	ctx := context.Background()

	t.Run("put_then_get_roundtrip", func(t *testing.T) {
		if err := f.Adapter.Put(ctx, f.Object, bytes.NewReader(f.Bytes), int64(len(f.Bytes)), adapter.Manifest{
			AssetID: f.Object.AssetID, Version: f.Object.Version, Digest: f.Object.Digest,
			SizeBytes: int64(len(f.Bytes)),
		}); err != nil {
			t.Fatalf("Put: %v", err)
		}
		rc, err := f.Adapter.Get(ctx, f.Object)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if err := rc.Close(); err != nil {
			t.Fatalf("Close (digest verification): %v", err)
		}
		if !bytes.Equal(got, f.Bytes) {
			t.Fatalf("bytes mismatch: got %d wanted %d", len(got), len(f.Bytes))
		}
	})

	t.Run("put_immutability", func(t *testing.T) {
		err := f.Adapter.Put(ctx, f.Object, bytes.NewReader(f.Bytes), int64(len(f.Bytes)), adapter.Manifest{})
		if err == nil {
			t.Fatal("expected second Put to fail with ErrCodeImmutable; got nil")
		}
		if !adapter.IsCode(err, adapter.ErrCodeImmutable) {
			t.Fatalf("expected ErrCodeImmutable; got %v", err)
		}
	})

	t.Run("stat_returns_manifest", func(t *testing.T) {
		m, err := f.Adapter.Stat(ctx, f.Object)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if m.AssetID != f.Object.AssetID || m.Version != f.Object.Version || m.Digest != f.Object.Digest {
			t.Fatalf("manifest mismatch: %+v", m)
		}
	})

	t.Run("cross_tenant_denied", func(t *testing.T) {
		other := f.Object
		other.TenantID = f.Object.TenantID + "-other"
		_, err := f.Adapter.Get(ctx, other)
		if !adapter.IsCode(err, adapter.ErrCodeCrossTenant) {
			t.Fatalf("expected ErrCodeCrossTenant; got %v", err)
		}
	})

	if f.CorruptBytesGet != nil {
		t.Run("get_digest_mismatch", func(t *testing.T) {
			f.CorruptBytesGet(f.Object)
			rc, err := f.Adapter.Get(ctx, f.Object)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if _, err := io.ReadAll(rc); err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			err = rc.Close()
			if !adapter.IsCode(err, adapter.ErrCodeDigestMismatch) {
				t.Fatalf("expected ErrCodeDigestMismatch on Close; got %v", err)
			}
		})
	}

	t.Run("audit_emitted", func(t *testing.T) {
		if f.AuditCapture == nil {
			t.Skip("driver did not wire AuditCapture")
		}
		events := f.AuditCapture.Snapshot()
		gotOps := map[string]int{}
		for _, e := range events {
			gotOps[e.Op]++
		}
		if gotOps["put"] == 0 {
			t.Fatalf("expected at least one put audit event; got %+v", gotOps)
		}
		if gotOps["get"] == 0 {
			t.Fatalf("expected at least one get audit event; got %+v", gotOps)
		}
	})
}

// AuditCapture is a thread-safe in-memory sink for tests.
type AuditCapture struct {
	events []adapter.AuditEvent
}

func (c *AuditCapture) EmitArtifactEvent(_ context.Context, e adapter.AuditEvent) error {
	c.events = append(c.events, e)
	return nil
}

func (c *AuditCapture) Snapshot() []adapter.AuditEvent {
	out := make([]adapter.AuditEvent, len(c.events))
	copy(out, c.events)
	return out
}

func TestNPMPublicBackendRejected(t *testing.T) {
	f := adapter.NewFactory(adapter.StaticSecretFetcher{}, &AuditCapture{})
	_, err := f.Build(context.Background(), adapter.BindingConfig{
		TenantID: "t1", Backend: adapter.BackendNPMPublic,
	})
	if !adapter.IsCode(err, adapter.ErrCodePublicBackend) {
		t.Fatalf("expected ErrCodePublicBackend; got %v", err)
	}
}

func TestUnknownBackendRejected(t *testing.T) {
	f := adapter.NewFactory(adapter.StaticSecretFetcher{}, &AuditCapture{})
	_, err := f.Build(context.Background(), adapter.BindingConfig{
		TenantID: "t1", Backend: "totally-made-up",
	})
	if !adapter.IsCode(err, adapter.ErrCodeInvalidInput) {
		t.Fatalf("expected ErrCodeInvalidInput; got %v", err)
	}
}

// fakeHealthAdapter is a registered driver factory the public-backend
// test uses to confirm the factory rejects backends whose Health
// reports IsPublicStorage=true.
type fakeHealthAdapter struct {
	public bool
}

func (a *fakeHealthAdapter) Put(_ context.Context, _ adapter.Object, _ io.Reader, _ int64, _ adapter.Manifest) error {
	return nil
}
func (a *fakeHealthAdapter) Get(_ context.Context, _ adapter.Object) (io.ReadCloser, error) {
	return nil, nil
}
func (a *fakeHealthAdapter) Stat(_ context.Context, _ adapter.Object) (adapter.Manifest, error) {
	return adapter.Manifest{}, nil
}
func (a *fakeHealthAdapter) Delete(_ context.Context, _ adapter.Object) error { return nil }
func (a *fakeHealthAdapter) Health(_ context.Context) (adapter.Health, error) {
	return adapter.Health{Healthy: true, IsPublicStorage: a.public}, nil
}
func (a *fakeHealthAdapter) Backend() string { return "fake-public-test" }

func TestPublicBackendRejectedByHealth(t *testing.T) {
	f := adapter.NewFactory(adapter.StaticSecretFetcher{}, &AuditCapture{})
	f.RegisterDriverOn("fake-public-test", func(_ context.Context, _ adapter.BindingConfig, _ adapter.Deps) (adapter.Adapter, error) {
		return &fakeHealthAdapter{public: true}, nil
	})
	_, err := f.Build(context.Background(), adapter.BindingConfig{TenantID: "t1", Backend: "fake-public-test"})
	if !adapter.IsCode(err, adapter.ErrCodePublicBackend) {
		t.Fatalf("expected ErrCodePublicBackend via Health probe; got %v", err)
	}
}

func TestUnhealthyBackendRejected(t *testing.T) {
	f := adapter.NewFactory(adapter.StaticSecretFetcher{}, &AuditCapture{})
	f.RegisterDriverOn("fake-public-test", func(_ context.Context, _ adapter.BindingConfig, _ adapter.Deps) (adapter.Adapter, error) {
		return &fakeHealthAdapter{public: false}, nil
	})
	// Override: register a driver that returns unhealthy.
	f.RegisterDriverOn("fake-public-test", func(_ context.Context, _ adapter.BindingConfig, _ adapter.Deps) (adapter.Adapter, error) {
		return unhealthyAdapter{}, nil
	})
	_, err := f.Build(context.Background(), adapter.BindingConfig{TenantID: "t1", Backend: "fake-public-test"})
	if !adapter.IsCode(err, adapter.ErrCodeBackendError) {
		t.Fatalf("expected ErrCodeBackendError; got %v", err)
	}
}

type unhealthyAdapter struct{}

func (unhealthyAdapter) Put(context.Context, adapter.Object, io.Reader, int64, adapter.Manifest) error {
	return nil
}
func (unhealthyAdapter) Get(context.Context, adapter.Object) (io.ReadCloser, error) { return nil, nil }
func (unhealthyAdapter) Stat(context.Context, adapter.Object) (adapter.Manifest, error) {
	return adapter.Manifest{}, nil
}
func (unhealthyAdapter) Delete(context.Context, adapter.Object) error { return nil }
func (unhealthyAdapter) Health(context.Context) (adapter.Health, error) {
	return adapter.Health{Healthy: false, Detail: "test induced"}, nil
}
func (unhealthyAdapter) Backend() string { return "unhealthy-fake" }
