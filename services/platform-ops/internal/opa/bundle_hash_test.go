package opa_test

import (
	"testing"

	"github.com/forge-eng-fabric/services/platform-ops/internal/opa"
)

func TestValidateBundleHash(t *testing.T) {
	// Build a client with a known bundle hash by writing a temporary .bundle-hash file.
	// Because New() reads ".bundle-hash" from the working directory we use a client
	// constructed via a thin helper that injects the hash directly.
	c := newClientWithHash(t, "sha256:abc123")

	tests := []struct {
		name     string
		rowHash  string
		wantOK   bool
	}{
		{"match", "sha256:abc123", true},
		{"mismatch", "sha256:other", false},
		{"empty row hash", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ValidateBundleHash(tt.rowHash)
			if got != tt.wantOK {
				t.Errorf("ValidateBundleHash(%q) = %v, want %v", tt.rowHash, got, tt.wantOK)
			}
		})
	}
}

func TestValidateBundleHashEmptyClient(t *testing.T) {
	// When the client has no bundle hash (no .bundle-hash file), any row hash
	// must be treated as a mismatch to avoid false positives.
	c := newClientWithHash(t, "")
	if c.ValidateBundleHash("sha256:anything") {
		t.Error("expected mismatch when client bundle hash is empty")
	}
}

// newClientWithHash creates a Client whose BundleHash() returns hash.
// It avoids touching the file system by using a bundleDir that doesn't exist —
// ValidateBundleHash does not load policies, it only compares hash strings.
func newClientWithHash(_ *testing.T, hash string) *opa.Client {
	return opa.NewWithHash("nonexistent-dir", hash)
}
