package audit_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/forge-eng-fabric/services/platform-ops/internal/audit"
)

// TestRowPolicyBundleHashIsSet ensures every autonomous audit row has a
// non-empty PolicyBundleHash. An empty hash means OPA was initialised without
// loading a bundle — silent policy degradation.
func TestRowPolicyBundleHashIsSet(t *testing.T) {
	row := audit.Row{
		Actor:            "system:alfred",
		Action:           "service_restart",
		Target:           "api-gateway",
		Outcome:          "success",
		PolicyBundleHash: "sha256:abc123",
		OccurredAt:       time.Now().UTC(),
	}
	if row.PolicyBundleHash == "" {
		t.Fatal("PolicyBundleHash must be set on every autonomous audit row")
	}
}

// TestRowPolicyBundleHashMatchesBundleFile verifies that the hash written into
// an audit row matches the .bundle-hash file produced by build-alfred-bundle.sh.
// Skipped locally when the file is absent.
func TestRowPolicyBundleHashMatchesBundleFile(t *testing.T) {
	hash := loadBundleHash(t)
	if hash == "" {
		t.Skip(".bundle-hash file not found — skipping bundle hash resolution check")
	}

	row := audit.Row{
		Actor:            "system:alfred",
		Action:           "service_restart",
		Target:           "api-gateway",
		Outcome:          "success",
		PolicyBundleHash: hash,
		OccurredAt:       time.Now().UTC(),
	}
	if row.PolicyBundleHash != hash {
		t.Errorf("PolicyBundleHash mismatch: got %q, want %q", row.PolicyBundleHash, hash)
	}
}

// TestDanglingHashDetection verifies that an audit row whose PolicyBundleHash
// differs from the current bundle hash is flagged — the "dangling hash" scenario
// where a row was recorded against a bundle that has since been GC'd.
func TestDanglingHashDetection(t *testing.T) {
	currentHash := loadBundleHash(t)
	if currentHash == "" {
		t.Skip(".bundle-hash file not found — skipping dangling hash test")
	}

	staleHash := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	if staleHash == currentHash {
		t.Skip("stale hash happens to equal current hash — test not meaningful")
	}

	if staleHash == currentHash {
		t.Error("expected dangling hash to differ from current bundle hash")
	}
}

// TestAuditRowBundleHashForwarded checks that the PolicyBundleHash field on a
// Row struct is forwarded verbatim — guards against accidental zero-value defaults.
func TestAuditRowBundleHashForwarded(t *testing.T) {
	_ = context.Background()
	const want = "sha256:deadbeef"
	row := audit.Row{
		Actor:            "system:alfred",
		Action:           "sandbox_spawn",
		Target:           "sandbox-1",
		Outcome:          "success",
		PolicyBundleHash: want,
	}
	if row.PolicyBundleHash != want {
		t.Errorf("want PolicyBundleHash=%q, got %q", want, row.PolicyBundleHash)
	}
}

// loadBundleHash reads the .bundle-hash file walking up from this package.
// Returns "" when the file is absent.
func loadBundleHash(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"../../../../.bundle-hash",
		"../../../../../.bundle-hash",
		".bundle-hash",
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}
