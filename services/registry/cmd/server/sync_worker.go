package main

// sync_worker.go — periodic public-origin sync worker (Task 13.6)
//
// SyncWorker walks every asset row with is_public_origin=true and
// auto_promote_policy != 'none', checks the upstream npm registry for a newer
// version, and triggers Mirror() when drift is detected.
//
// Rate-limited to ≤100 requests/min via a 600 ms sleep between npm registry
// calls. At cycle end emits registry.sync.completed.v1.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

// SyncResult summarises one full sync cycle across all tenants.
type SyncResult struct {
	TenantID string
	Checked  int
	Drifted  int
	Mirrored int
	Errors   int
}

// SyncWorker periodically checks public-origin assets for drift vs their
// upstream registries.
//
// Configurable via SYNC_WORKER_INTERVAL env var (default: 168h = weekly).
// Rate-limited to ≤100 requests/min per origin via a 600 ms inter-request sleep.
type SyncWorker struct {
	srv      *server
	interval time.Duration
}

// NewSyncWorker creates a SyncWorker. If interval ≤ 0, defaults to 168 h.
func NewSyncWorker(srv *server, interval time.Duration) *SyncWorker {
	if interval <= 0 {
		interval = 168 * time.Hour
	}
	return &SyncWorker{srv: srv, interval: interval}
}

// Run is the main loop. Blocks until ctx is cancelled.
func (w *SyncWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runCycle(ctx)
		}
	}
}

// syncRow is one candidate asset returned by the DB query.
type syncRow struct {
	id           string
	tenantID     uuid.UUID
	originRef    string
	lastSyncedAt *time.Time
	version      string // latest stored version
}

// runCycle executes one full pass over all public-origin assets.
func (w *SyncWorker) runCycle(ctx context.Context) {
	result := SyncResult{}

	// 1. List all public-origin assets with auto-promotion enabled.
	//    We select the latest version per asset id using a window function so we
	//    only probe each upstream package once per cycle.
	rows, err := w.srv.pool.Query(ctx,
		`SELECT DISTINCT ON (id) id, tenant_id, COALESCE(origin_ref,''), last_synced_at, version
		   FROM asset
		  WHERE is_public_origin = true
		    AND auto_promote_policy != 'none'
		  ORDER BY id, created_at DESC`)
	if err != nil {
		log.Printf("sync_worker: list public-origin assets: %v", err)
		return
	}

	var candidates []syncRow
	for rows.Next() {
		var row syncRow
		if err := rows.Scan(&row.id, &row.tenantID, &row.originRef, &row.lastSyncedAt, &row.version); err != nil {
			log.Printf("sync_worker: scan row: %v", err)
			result.Errors++
			continue
		}
		candidates = append(candidates, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("sync_worker: iterate rows: %v", err)
	}

	result.Checked = len(candidates)

	// 2. For each candidate, probe the upstream registry and mirror if needed.
	for _, row := range candidates {
		if ctx.Err() != nil {
			break
		}

		// Rate-limit: ~100 req/min → 600 ms between requests.
		time.Sleep(600 * time.Millisecond)

		if err := w.processAsset(ctx, row, &result); err != nil {
			log.Printf("sync_worker: asset %s: %v", row.id, err)
			result.Errors++
		}
	}

	// 3. Emit registry.sync.completed.v1.
	w.publishSyncCompleted(ctx, result)

	log.Printf("sync_worker: cycle done checked=%d drifted=%d mirrored=%d errors=%d",
		result.Checked, result.Drifted, result.Mirrored, result.Errors)
}

// processAsset probes the upstream for one asset and triggers Mirror on drift.
func (w *SyncWorker) processAsset(ctx context.Context, row syncRow, result *SyncResult) error {
	// Only npm origins are supported; log and skip others.
	pkg, _, err := parseNPMOriginRef(row.originRef)
	if err != nil {
		log.Printf("sync_worker: asset %s: non-npm origin %q, skipping: %v", row.id, row.originRef, err)
		return nil
	}

	// 2a. Fetch the latest version from npm.
	upstreamVersion, sha256, err := npmLatestVersion(ctx, pkg)
	if err != nil {
		return fmt.Errorf("npm latest %s: %w", pkg, err)
	}

	// 2b. Compare with the stored latest version (already in row.version).
	if upstreamVersion == row.version {
		return nil // no drift
	}

	result.Drifted++

	// 2c. Check if we already have this version stored.
	var count int
	if scanErr := w.srv.pool.QueryRow(ctx,
		`SELECT count(*) FROM asset WHERE id=$1 AND version=$2`,
		row.id, upstreamVersion).Scan(&count); scanErr != nil {
		return fmt.Errorf("check existing version: %w", scanErr)
	}
	if count > 0 {
		// Already mirrored, skip.
		return nil
	}

	// 2d. Trigger Mirror for the new version.
	mirrorReq := MirrorRequest{
		OriginRef:           fmt.Sprintf("npm:%s@%s", pkg, upstreamVersion),
		SHA256:              sha256,
		TenantID:            row.tenantID,
		AssetID:             row.id,
		Version:             upstreamVersion,
		PublicOriginEnabled: true,
	}
	if _, mirrorErr := w.srv.Mirror(ctx, mirrorReq); mirrorErr != nil {
		return fmt.Errorf("mirror %s@%s: %w", row.id, upstreamVersion, mirrorErr)
	}

	result.Mirrored++
	return nil
}

// npmLatestVersion queries registry.npmjs.org for the latest published version
// and its sha256 tarball digest. Returns (version, sha256hex, error).
func npmLatestVersion(ctx context.Context, pkg string) (string, string, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/%s/latest", pkg)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("npm registry responded %d for %s", resp.StatusCode, url)
	}

	var meta struct {
		Version string `json:"version"`
		Dist    struct {
			Shasum  string `json:"shasum"`  // sha1 — present in all npm responses
			// integrity field contains sha512 in SRI format; npm doesn't expose
			// sha256 directly in the latest metadata endpoint, so we fall back
			// to requesting the tarball tarball and computing it in Mirror().
			// We pass an empty string here; Mirror() will compute the digest.
			Integrity string `json:"integrity"`
		} `json:"dist"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", "", fmt.Errorf("decode npm response: %w", err)
	}
	if meta.Version == "" {
		return "", "", fmt.Errorf("npm response missing version for %s", pkg)
	}

	// Mirror() performs its own sha256 verification by computing the digest from
	// the fetched bytes and comparing. When the sync worker calls Mirror() it
	// passes an empty sha256 field; Mirror() should be tolerant of this.
	// For the sync path, we set SHA256 to "" so Mirror() skips the pre-check
	// and computes it from the downloaded bytes directly.
	return meta.Version, "", nil
}

// publishSyncCompleted emits a com.forge.registry.sync.completed.v1 CloudEvent.
func (w *SyncWorker) publishSyncCompleted(ctx context.Context, result SyncResult) {
	envelope := map[string]any{
		"specversion":     "1.0",
		"id":              uuid.NewString(),
		"source":          "forge://service/registry/sync-worker",
		"type":            "com.forge.registry.sync.completed.v1",
		"subject":         "sync/cycle",
		"time":            time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype": "application/json",
		"data": map[string]any{
			"tenant_id": result.TenantID,
			"checked":   result.Checked,
			"drifted":   result.Drifted,
			"mirrored":  result.Mirrored,
			"errors":    result.Errors,
		},
	}
	body, _ := json.Marshal(envelope)
	_ = w.srv.kc.ProduceSync(ctx, &kgo.Record{
		Topic: w.srv.topic,
		Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte("com.forge.registry.sync.completed.v1")},
			{Key: "content-type", Value: []byte("application/cloudevents+json")},
		},
	}).FirstErr()
}

// syncWorkerIntervalFromEnv reads SYNC_WORKER_INTERVAL from the environment.
// Returns 168h on parse failure.
func syncWorkerIntervalFromEnv() time.Duration {
	raw := os.Getenv("SYNC_WORKER_INTERVAL")
	if raw == "" {
		return 168 * time.Hour
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Printf("sync_worker: SYNC_WORKER_INTERVAL %q invalid, defaulting to 168h", raw)
		return 168 * time.Hour
	}
	return d
}
