package adapter_test

import (
	"context"
	"io"
	"testing"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
)

// publicStorageAdapter always reports IsPublicStorage=true, simulating a
// misconfigured backend that is publicly accessible.
type publicStorageAdapter struct{}

func (publicStorageAdapter) Put(context.Context, adapter.Object, io.Reader, int64, adapter.Manifest) error {
	return nil
}
func (publicStorageAdapter) Get(context.Context, adapter.Object) (io.ReadCloser, error) { return nil, nil }
func (publicStorageAdapter) Stat(context.Context, adapter.Object) (adapter.Manifest, error) {
	return adapter.Manifest{}, nil
}
func (publicStorageAdapter) Delete(context.Context, adapter.Object) error { return nil }
func (publicStorageAdapter) Health(context.Context) (adapter.Health, error) {
	return adapter.Health{Healthy: true, IsPublicStorage: true}, nil
}
func (publicStorageAdapter) Backend() string { return "test-public-storage" }

// publicOriginPrivateStorageAdapter reports IsPublicOrigin=true but
// IsPublicStorage=false, modelling a mirrored npm package stored in a private
// backend (the valid public-origin mirror flow).
type publicOriginPrivateStorageAdapter struct{}

func (publicOriginPrivateStorageAdapter) Put(context.Context, adapter.Object, io.Reader, int64, adapter.Manifest) error {
	return nil
}
func (publicOriginPrivateStorageAdapter) Get(context.Context, adapter.Object) (io.ReadCloser, error) {
	return nil, nil
}
func (publicOriginPrivateStorageAdapter) Stat(context.Context, adapter.Object) (adapter.Manifest, error) {
	return adapter.Manifest{}, nil
}
func (publicOriginPrivateStorageAdapter) Delete(context.Context, adapter.Object) error { return nil }
func (publicOriginPrivateStorageAdapter) Health(context.Context) (adapter.Health, error) {
	return adapter.Health{
		Healthy:         true,
		IsPublicOrigin:  true,  // came from npm/GitHub public; fine to store here
		IsPublicStorage: false, // the storage backend itself is private
	}, nil
}
func (publicOriginPrivateStorageAdapter) Backend() string { return "test-public-origin-private-storage" }

// TestIsPublicStorage_Rejected verifies that Factory.Build returns
// ErrCodePublicBackend when the driver Health reports IsPublicStorage=true,
// even though the backend is otherwise healthy.
func TestIsPublicStorage_Rejected(t *testing.T) {
	f := adapter.NewFactory(adapter.StaticSecretFetcher{}, &AuditCapture{})
	f.RegisterDriverOn("test-public-storage", func(_ context.Context, _ adapter.BindingConfig, _ adapter.Deps) (adapter.Adapter, error) {
		return publicStorageAdapter{}, nil
	})
	_, err := f.Build(context.Background(), adapter.BindingConfig{TenantID: "t1", Backend: "test-public-storage"})
	if !adapter.IsCode(err, adapter.ErrCodePublicBackend) {
		t.Fatalf("expected ErrCodePublicBackend when IsPublicStorage=true; got %v", err)
	}
}

// TestIsPublicOrigin_PrivateStorage_Allowed verifies that Factory.Build
// succeeds when the driver reports IsPublicOrigin=true but IsPublicStorage=false.
// Public-origin assets (mirrored from npm, GitHub public) are allowed to be
// stored in private backends — this is the "public origin mirror flow".
func TestIsPublicOrigin_PrivateStorage_Allowed(t *testing.T) {
	f := adapter.NewFactory(adapter.StaticSecretFetcher{}, &AuditCapture{})
	f.RegisterDriverOn("test-public-origin-private-storage", func(_ context.Context, _ adapter.BindingConfig, _ adapter.Deps) (adapter.Adapter, error) {
		return publicOriginPrivateStorageAdapter{}, nil
	})
	got, err := f.Build(context.Background(), adapter.BindingConfig{TenantID: "t1", Backend: "test-public-origin-private-storage"})
	if err != nil {
		t.Fatalf("expected Build to succeed for public-origin/private-storage adapter; got error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil adapter")
	}
}
