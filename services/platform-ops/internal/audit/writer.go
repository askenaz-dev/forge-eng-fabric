// Package audit writes autonomous action audit rows to the audit table.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Row represents a platform-ops audit event.
type Row struct {
	AuditID          string
	TenantID         string
	Actor            string
	ActorSession     string
	Action           string
	Target           string
	Outcome          string
	SymptomID        string
	SessionID        string
	PolicyBundleHash string
	Verification     map[string]any
	RollbackActionID string
	Details          map[string]any
	OccurredAt       time.Time
}

// Writer persists audit rows to PostgreSQL.
type Writer struct {
	pool *pgxpool.Pool
}

// New creates a Writer.
func New(pool *pgxpool.Pool) *Writer { return &Writer{pool: pool} }

// Write inserts an audit row. Returns the assigned audit_id.
func (w *Writer) Write(ctx context.Context, r Row) (string, error) {
	if r.AuditID == "" {
		r.AuditID = uuid.NewString()
	}
	if r.OccurredAt.IsZero() {
		r.OccurredAt = time.Now().UTC()
	}

	detailsJSON, _ := json.Marshal(r.Details)
	verJSON, _ := json.Marshal(r.Verification)

	_, err := w.pool.Exec(ctx, `
		INSERT INTO platform_ops_audit_event (
			audit_id, tenant_id, actor, actor_session, action, resource, outcome,
			details, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`,
		r.AuditID, nilUUID(r.TenantID), r.Actor, r.ActorSession,
		r.Action, r.Target, r.Outcome,
		json.RawMessage(detailsJSON), r.OccurredAt,
	)
	if err != nil {
		return "", fmt.Errorf("audit write: %w", err)
	}

	// Supplemental autonomous fields written to platform_ops_audit_ext.
	_, _ = w.pool.Exec(ctx, `
		INSERT INTO platform_ops_audit_ext (
			audit_id, symptom_id, agent_session_id, policy_bundle_hash,
			verification, rollback_action_id
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (audit_id) DO NOTHING
	`,
		r.AuditID, nilUUID(r.SymptomID), nilUUID(r.SessionID), r.PolicyBundleHash,
		json.RawMessage(verJSON), nilUUID(r.RollbackActionID),
	)

	return r.AuditID, nil
}

// nilUUID converts an empty string to nil so pgx treats it as NULL for uuid columns.
func nilUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}
