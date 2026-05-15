-- +goose Up
-- Phase 5: add nullable app_id to openspec_index so OpenSpecs can be re-anchored
-- under their parent App. The column is filled by the backfill job and flipped
-- to NOT NULL (and CHECK-constrained against application.workspace_id) by
-- db/migrations/openspec/0003_app_id_not_null.sql once per-workspace sentinels
-- are set in registry.application_backfill_sentinel.
--
-- Note: the FK to registry.application(id) is intentionally not declared here;
-- the openspec service runs on its own logical schema and we keep the column as
-- a bare uuid validated at the app layer. The DB-level FK is added in the
-- registry migration after the backfill completes (see 0009 in registry).

ALTER TABLE openspec_index
  ADD COLUMN IF NOT EXISTS app_id uuid;

CREATE INDEX IF NOT EXISTS openspec_index_app_id_idx
  ON openspec_index (app_id);

-- +goose Down
DROP INDEX IF EXISTS openspec_index_app_id_idx;
ALTER TABLE openspec_index DROP COLUMN IF EXISTS app_id;
