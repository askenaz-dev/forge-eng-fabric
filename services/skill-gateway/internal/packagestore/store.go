// Package packagestore is the thin abstraction over the object store where
// packaged Agent Skills bundles live. The gateway reads bytes from here when
// serving the package download endpoint and can choose to redirect to a
// pre-signed URL for large bundles instead of streaming them inline.
package packagestore

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
)

// Store is the interface the gateway requires from its object store. A real
// implementation wraps S3 / GCS / Azure Blob; an in-memory implementation is
// shipped here for tests and local dev.
type Store interface {
	// Get returns a reader for the bundle bytes addressed by bytesURI.
	Get(ctx context.Context, bytesURI string) (io.ReadCloser, int64, error)
	// PresignedURL returns a short-lived URL the client can fetch directly,
	// or an empty string when this store does not support redirects.
	PresignedURL(ctx context.Context, bytesURI string, ttlSeconds int) (string, error)
}

// MemoryStore is a process-local store. Keys are the canonical bytesURI.
type MemoryStore struct {
	Bundles map[string][]byte
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{Bundles: map[string][]byte{}} }

func (m *MemoryStore) Get(_ context.Context, bytesURI string) (io.ReadCloser, int64, error) {
	body, ok := m.Bundles[bytesURI]
	if !ok {
		return nil, 0, ErrNotFound
	}
	return io.NopCloser(strings.NewReader(string(body))), int64(len(body)), nil
}

func (m *MemoryStore) PresignedURL(_ context.Context, _ string, _ int) (string, error) {
	return "", nil
}

// ParseBytesURI validates the canonical `s3://bucket/key` / `gs://...` shape
// and returns scheme + host + path. Used by future S3/GCS implementations.
func ParseBytesURI(raw string) (scheme, host, path string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", "", errors.New("bytes_uri must be of the form <scheme>://<host>/<path>")
	}
	return u.Scheme, u.Host, strings.TrimPrefix(u.Path, "/"), nil
}

var ErrNotFound = errors.New("package_not_found")
