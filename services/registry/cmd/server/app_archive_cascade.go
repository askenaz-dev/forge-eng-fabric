package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// cascadeAppArchive marks every asset attached to `appID` as `deprecated` in
// response to an `app.archived.v1` event (app-first-class-entity 8.6). Assets
// in terminal states (`retired`) are skipped. The function emits one
// `asset.discoverability.changed.v1` audit event per affected asset, with the
// originating `correlationID` so downstream pipelines can join the trail.
//
// The function is idempotent: rerunning it for the same App is a no-op once
// every active asset has flipped to deprecated. Production wires this to the
// platform event bus; the registry service's main loop calls it when it
// receives the lifecycle event from the application service.
func cascadeAppArchive(
	ctx context.Context,
	pool *pgxpool.Pool,
	emit func(eventType, subject string, data map[string]any),
	appID uuid.UUID,
	correlationID string,
) (int, error) {
	if pool == nil {
		return 0, fmt.Errorf("pool is required")
	}
	rows, err := pool.Query(ctx,
		`UPDATE asset
		   SET lifecycle_state='deprecated'
		 WHERE app_id=$1 AND lifecycle_state NOT IN ('deprecated','retired')
		 RETURNING id, version, workspace_id, tenant_id`,
		appID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, version string
		var wsID, tenantID uuid.UUID
		if err := rows.Scan(&id, &version, &wsID, &tenantID); err != nil {
			return count, err
		}
		count++
		if emit != nil {
			emit("asset.discoverability.changed.v1", fmt.Sprintf("asset/%s@%s", id, version), map[string]any{
				"asset_id":       id,
				"version":        version,
				"workspace_id":   wsID.String(),
				"tenant_id":      tenantID.String(),
				"app_id":         appID.String(),
				"discoverable":   false,
				"reason":         "app_archived",
				"correlation_id": correlationID,
			})
		}
	}
	return count, rows.Err()
}
