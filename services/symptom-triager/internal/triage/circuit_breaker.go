package triage

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	cbMaxFailures = 3
	cbCooldown    = 30 * time.Minute
)

// CircuitBreakerStore manages per-fingerprint circuit-breaker state in Postgres.
type CircuitBreakerStore struct {
	pool *pgxpool.Pool
}

// NewCircuitBreakerStore creates a store.
func NewCircuitBreakerStore(pool *pgxpool.Pool) *CircuitBreakerStore {
	return &CircuitBreakerStore{pool: pool}
}

// IsOpen returns true if the circuit breaker for the given fingerprint is open.
func (s *CircuitBreakerStore) IsOpen(ctx context.Context, fp string) (bool, error) {
	var isOpen bool
	var cooldownUntil *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT is_open, cooldown_until
		FROM circuit_breaker_state
		WHERE fingerprint = $1
	`, fp).Scan(&isOpen, &cooldownUntil)
	if err != nil {
		// No row means no breaker state — breaker is closed.
		return false, nil
	}
	if isOpen && cooldownUntil != nil && time.Now().After(*cooldownUntil) {
		// Auto-close after cooldown.
		_, _ = s.pool.Exec(ctx, `
			UPDATE circuit_breaker_state
			SET is_open=false, failed_session_count=0
			WHERE fingerprint=$1
		`, fp)
		return false, nil
	}
	return isOpen, nil
}

// RecordFailure increments the failure counter for a fingerprint and opens the
// circuit breaker after cbMaxFailures consecutive failures.
func (s *CircuitBreakerStore) RecordFailure(ctx context.Context, fp string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO circuit_breaker_state (fingerprint, failed_session_count, is_open, opened_at, cooldown_until)
		VALUES ($1, 1, false, NULL, NULL)
		ON CONFLICT (fingerprint) DO UPDATE
		  SET failed_session_count = circuit_breaker_state.failed_session_count + 1
	`, fp)
	if err != nil {
		return err
	}

	var count int
	_ = s.pool.QueryRow(ctx, `SELECT failed_session_count FROM circuit_breaker_state WHERE fingerprint=$1`, fp).Scan(&count)

	if count >= cbMaxFailures {
		cooldown := time.Now().Add(cbCooldown)
		_, _ = s.pool.Exec(ctx, `
			UPDATE circuit_breaker_state
			SET is_open=true, opened_at=now(), cooldown_until=$2
			WHERE fingerprint=$1 AND NOT is_open
		`, fp, cooldown)
		slog.Warn("circuit breaker opened",
			"fingerprint", fp,
			"failures", count,
			"cooldown_until", cooldown.Format(time.RFC3339),
		)
	}
	return nil
}

// RecordSuccess resets the failure streak for a fingerprint.
func (s *CircuitBreakerStore) RecordSuccess(ctx context.Context, fp string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO circuit_breaker_state (fingerprint, failed_session_count, is_open)
		VALUES ($1, 0, false)
		ON CONFLICT (fingerprint) DO UPDATE SET failed_session_count=0
	`, fp)
	return err
}
