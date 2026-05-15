package drift

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeFetcher returns a fixed hash regardless of endpoint. Tests inject
// different fakes for the drift-detected vs. drift-clean cases.
type fakeFetcher struct {
	hash string
	err  error
}

func (f fakeFetcher) FetchManifest(_ context.Context, _ string) (string, error) {
	return f.hash, f.err
}
func (f fakeFetcher) FetchAgentCard(_ context.Context, _ string) (string, error) {
	return f.hash, f.err
}

// capturePublisher records emitted events for assertion.
type capturePublisher struct {
	mu     sync.Mutex
	events []capturedEvent
}

type capturedEvent struct {
	Type    string
	Subject string
	Data    map[string]any
}

func (c *capturePublisher) Publish(_ context.Context, eventType string, _ []byte, body []byte) error {
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		return err
	}
	subj, _ := env["subject"].(string)
	data, _ := env["data"].(map[string]any)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, capturedEvent{Type: eventType, Subject: subj, Data: data})
	return nil
}

func (c *capturePublisher) byType(t string) []capturedEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []capturedEvent
	for _, e := range c.events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

// dbPoolOrSkip dials Postgres at $POSTGRES_URL (defaults to docker-compose
// loopback) and skips the test if unreachable. The drift cron is genuinely
// stateful so a real DB is the cheapest fixture.
func dbPoolOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = "postgres://forge:forge@localhost:15432/forge_registry?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skipf("skip: cannot create pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skip: cannot ping db at %s: %v", url, err)
	}
	// Verify migration 0007 columns exist; the test fixture inserts rows
	// that reference them, so if 0007 is unapplied we cannot test.
	var has bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='asset' AND column_name='external_provenance')`).
		Scan(&has); err != nil || !has {
		pool.Close()
		t.Skipf("skip: migration 0007 not applied: has_external_provenance=%v err=%v", has, err)
	}
	return pool
}

// fixture creates an asset row and the matching external_mcp_endpoint row,
// returns a cleanup func. Each test gets a unique asset id so concurrent
// runs do not collide.
func fixture(t *testing.T, pool *pgxpool.Pool, lifecycle, storedHash string) (assetID, version string, cleanup func()) {
	t.Helper()
	ctx := context.Background()
	assetID = "test-drift:" + uuid.NewString()
	version = "0.1.0"
	tenant := uuid.New()
	workspace := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO asset(id, version, type, name, owner_team, inputs_schema, outputs_schema, workspace_id, tenant_id, visibility, lifecycle_state, trust_level, external_provenance)
		 VALUES ($1,$2,'mcp','drift-fixture','drift-test','{}'::jsonb,'{}'::jsonb,$3,$4,'workspace',$5,'T1','external')`,
		assetID, version, workspace, tenant, lifecycle); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO external_mcp_endpoint(asset_id, tenant_id, endpoint_url, credential_ref, manifest_hash, manifest_fetched_at, created_by)
		 VALUES ($1,$2,'https://example.com/mcp','vault://t/secret',$3,now() - interval '7 days','drift-test')`,
		assetID, tenant, storedHash); err != nil {
		t.Fatalf("insert endpoint: %v", err)
	}
	cleanup = func() {
		_, _ = pool.Exec(ctx, `DELETE FROM external_mcp_endpoint WHERE asset_id=$1`, assetID)
		_, _ = pool.Exec(ctx, `DELETE FROM asset_lifecycle_event WHERE asset_id=$1`, assetID)
		_, _ = pool.Exec(ctx, `DELETE FROM asset WHERE id=$1`, assetID)
	}
	return assetID, version, cleanup
}

func TestDriftDetectedDeprecatesApprovedAsset(t *testing.T) {
	pool := dbPoolOrSkip(t)
	defer pool.Close()

	assetID, _, cleanup := fixture(t, pool, "approved", "sha256:old")
	defer cleanup()

	pub := &capturePublisher{}
	runner := &Runner{
		Pool:      pool,
		MCP:       fakeFetcher{hash: "sha256:new-drifted"},
		A2A:       fakeFetcher{hash: "sha256:unused"},
		Publisher: pub,
		Interval:  time.Hour,
	}
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Drift event emitted.
	if got := pub.byType("com.forge.asset.external_drift.v1"); len(got) != 1 || got[0].Data["live_hash"] != "sha256:new-drifted" {
		t.Fatalf("expected one drift event with new hash; got %+v", got)
	}
	// Auto-deprecation event emitted because the asset was approved.
	if got := pub.byType("com.forge.asset.external_drift_deprecated.v1"); len(got) != 1 {
		t.Fatalf("expected one auto-deprecate event; got %+v", got)
	}

	// Asset moved to deprecated.
	var lifecycle string
	if err := pool.QueryRow(context.Background(),
		`SELECT lifecycle_state FROM asset WHERE id=$1`, assetID).Scan(&lifecycle); err != nil {
		t.Fatalf("re-read asset: %v", err)
	}
	if lifecycle != "deprecated" {
		t.Fatalf("expected deprecated; got %s", lifecycle)
	}
}

func TestDriftCleanNoDeprecation(t *testing.T) {
	pool := dbPoolOrSkip(t)
	defer pool.Close()

	hash := "sha256:stable"
	assetID, _, cleanup := fixture(t, pool, "approved", hash)
	defer cleanup()

	pub := &capturePublisher{}
	runner := &Runner{
		Pool:      pool,
		MCP:       fakeFetcher{hash: hash},
		A2A:       fakeFetcher{hash: "sha256:unused"},
		Publisher: pub,
		Interval:  time.Hour,
	}
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := pub.byType("com.forge.asset.external_drift.v1"); len(got) != 0 {
		t.Fatalf("expected no drift event; got %+v", got)
	}
	var lifecycle string
	_ = pool.QueryRow(context.Background(),
		`SELECT lifecycle_state FROM asset WHERE id=$1`, assetID).Scan(&lifecycle)
	if lifecycle != "approved" {
		t.Fatalf("expected approved (unchanged); got %s", lifecycle)
	}
}

func TestDriftInReviewEmitsButDoesNotDeprecate(t *testing.T) {
	pool := dbPoolOrSkip(t)
	defer pool.Close()
	assetID, _, cleanup := fixture(t, pool, "in_review", "sha256:old")
	defer cleanup()
	pub := &capturePublisher{}
	runner := &Runner{
		Pool: pool, MCP: fakeFetcher{hash: "sha256:new"}, A2A: fakeFetcher{hash: "sha256:n/a"},
		Publisher: pub, Interval: time.Hour,
	}
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := pub.byType("com.forge.asset.external_drift.v1"); len(got) != 1 {
		t.Fatalf("expected one drift event for in_review; got %+v", got)
	}
	if got := pub.byType("com.forge.asset.external_drift_deprecated.v1"); len(got) != 0 {
		t.Fatalf("in_review should NOT auto-deprecate; got %+v", got)
	}
	var lifecycle string
	_ = pool.QueryRow(context.Background(),
		`SELECT lifecycle_state FROM asset WHERE id=$1`, assetID).Scan(&lifecycle)
	if lifecycle != "in_review" {
		t.Fatalf("expected in_review unchanged; got %s", lifecycle)
	}
}

func TestDriftFetchErrorEmitsCheckFailedNotDrift(t *testing.T) {
	pool := dbPoolOrSkip(t)
	defer pool.Close()
	assetID, _, cleanup := fixture(t, pool, "approved", "sha256:old")
	defer cleanup()
	_ = assetID
	pub := &capturePublisher{}
	runner := &Runner{
		Pool: pool, MCP: fakeFetcher{err: errors.New("upstream timeout")},
		A2A: fakeFetcher{hash: "sha256:n/a"}, Publisher: pub, Interval: time.Hour,
	}
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := pub.byType("com.forge.asset.external_drift_check_failed.v1"); len(got) != 1 {
		t.Fatalf("expected one drift-check-failed; got %+v", got)
	}
	if got := pub.byType("com.forge.asset.external_drift.v1"); len(got) != 0 {
		t.Fatalf("transient fetch error must not emit drift; got %+v", got)
	}
	if got := pub.byType("com.forge.asset.external_drift_deprecated.v1"); len(got) != 0 {
		t.Fatalf("transient fetch error must not auto-deprecate; got %+v", got)
	}
}

func TestFetchRowsForFamilyExcludesDeprecated(t *testing.T) {
	pool := dbPoolOrSkip(t)
	defer pool.Close()
	approvedID, _, cleanup1 := fixture(t, pool, "approved", "sha256:x")
	defer cleanup1()
	deprID, _, cleanup2 := fixture(t, pool, "deprecated", "sha256:y")
	defer cleanup2()

	runner := &Runner{Pool: pool, Interval: time.Hour}
	ids, err := runner.FetchRowsForFamily(context.Background(), "mcp")
	if err != nil {
		t.Fatalf("FetchRowsForFamily: %v", err)
	}
	want := map[string]bool{approvedID: true}
	got := map[string]bool{}
	for _, id := range ids {
		// Only collect ids from this test's fixtures; live data may include others.
		if id == approvedID || id == deprID {
			got[id] = true
		}
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("expected %q in candidates; got %v", k, got)
		}
	}
	if got[deprID] {
		t.Fatalf("deprecated asset should not appear in drift candidates")
	}
}
