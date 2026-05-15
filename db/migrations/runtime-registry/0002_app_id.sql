-- +goose Up
-- Phase 5: add nullable app_id to runtime so a runtime registration can be
-- anchored to its parent App. Backfill is workspace-scoped — runtimes that
-- only carry workspace_id today are migrated to `_unassigned` when no
-- explicit App candidate exists.

ALTER TABLE runtime
  ADD COLUMN IF NOT EXISTS app_id uuid;

CREATE INDEX IF NOT EXISTS runtime_app_id_idx
  ON runtime (app_id);

-- +goose Down
DROP INDEX IF EXISTS runtime_app_id_idx;
ALTER TABLE runtime DROP COLUMN IF EXISTS app_id;
