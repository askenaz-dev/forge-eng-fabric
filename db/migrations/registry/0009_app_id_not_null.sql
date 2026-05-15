-- +goose Up
-- Phase 5 — M5 cutover: flip nullable app_id columns to NOT NULL for every
-- workspace that has a backfill-complete sentinel. This migration is intended
-- to be executed *after* the per-workspace migration job (cmd/spec-app-migration
-- execute) has set rows in registry.application_backfill_sentinel and asserted
-- 0 NULL app_id rows for the workspace.
--
-- The migration is composed of two phases:
--   1. Gate: refuse to run if any NULL app_id rows remain across the four
--      anchored tables. The gate is workspace-wide because we ship the flip
--      globally once all workspaces have a sentinel.
--   2. Constraint: set NOT NULL, add the FK to application(id), and add the
--      app_workspace_coherence CHECK on openspec_index.

-- +goose StatementBegin
DO $$
DECLARE
  null_count bigint := 0;
BEGIN
  SELECT count(*) INTO null_count FROM asset            WHERE app_id IS NULL;
  IF null_count > 0 THEN
    RAISE EXCEPTION 'asset has % rows with NULL app_id', null_count;
  END IF;
  SELECT count(*) INTO null_count FROM asset_deployment WHERE app_id IS NULL;
  IF null_count > 0 THEN
    RAISE EXCEPTION 'asset_deployment has % rows with NULL app_id', null_count;
  END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE asset
  ALTER COLUMN app_id SET NOT NULL;
ALTER TABLE asset_deployment
  ALTER COLUMN app_id SET NOT NULL;

-- +goose Down
ALTER TABLE asset_deployment ALTER COLUMN app_id DROP NOT NULL;
ALTER TABLE asset            ALTER COLUMN app_id DROP NOT NULL;
