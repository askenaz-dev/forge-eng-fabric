// Audit archiver exports old audit rows to local storage or GCS.
// It never deletes rows because audit_event is append-only in Phase 0.
package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type auditRow struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	WorkspaceID   *string         `json:"workspace_id,omitempty"`
	Actor         string          `json:"actor"`
	Action        string          `json:"action"`
	Resource      string          `json:"resource"`
	Outcome       string          `json:"outcome"`
	Details       json.RawMessage `json:"details"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	PrevHash      string          `json:"prev_hash"`
	Hash          string          `json:"hash"`
	OccurredAt    time.Time       `json:"occurred_at"`
}

func main() {
	ctx := context.Background()
	postgresURL := getenv("POSTGRES_URL", "postgres://forge:forge@localhost:5432/forge_audit?sslmode=disable")
	archiveURI := getenv("AUDIT_ARCHIVE_URI", "file://./var/audit-archive")
	retentionDays := getenvInt("AUDIT_RETENTION_DAYS", 365)
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)

	pool, err := pgxpool.New(ctx, postgresURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	path, err := exportRows(ctx, pool, cutoff)
	if err != nil {
		log.Fatalf("export: %v", err)
	}
	if err := publishArchive(ctx, path, archiveURI); err != nil {
		log.Fatalf("publish archive: %v", err)
	}
	log.Printf("archived audit rows older than %s to %s", cutoff.Format(time.RFC3339), archiveURI)
}

func exportRows(ctx context.Context, pool *pgxpool.Pool, cutoff time.Time) (string, error) {
	if err := os.MkdirAll(filepath.Join("var", "audit-archive"), 0o755); err != nil {
		return "", err
	}
	name := filepath.Join("var", "audit-archive", "audit-"+cutoff.Format("20060102T150405Z")+".ndjson.gz")
	file, err := os.Create(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	writer := bufio.NewWriter(gz)
	defer writer.Flush()

	rows, err := pool.Query(ctx, `SELECT id::text, tenant_id::text, workspace_id::text, actor, action, resource, outcome, details, COALESCE(correlation_id,''), prev_hash, hash, occurred_at FROM audit_event WHERE occurred_at < $1 ORDER BY occurred_at, id`, cutoff)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var row auditRow
		if err := rows.Scan(&row.ID, &row.TenantID, &row.WorkspaceID, &row.Actor, &row.Action, &row.Resource, &row.Outcome, &row.Details, &row.CorrelationID, &row.PrevHash, &row.Hash, &row.OccurredAt); err != nil {
			return "", err
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return "", err
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return "", err
		}
	}
	return name, rows.Err()
}

func publishArchive(ctx context.Context, src, dst string) error {
	if strings.HasPrefix(dst, "gs://") {
		cmd := exec.CommandContext(ctx, "gcloud", "storage", "cp", src, strings.TrimRight(dst, "/")+"/"+filepath.Base(src))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	u, err := url.Parse(dst)
	if err != nil {
		return err
	}
	dir := dst
	if u.Scheme == "file" {
		dir = u.Path
		if dir == "" {
			dir = u.Host
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return copyFile(src, filepath.Join(dir, filepath.Base(src)))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.ReadFrom(in)
	return err
}

func getenv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func getenvInt(name string, fallback int) int {
	if value := os.Getenv(name); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
		fmt.Fprintf(os.Stderr, "invalid %s=%q, using %d\n", name, value, fallback)
	}
	return fallback
}
