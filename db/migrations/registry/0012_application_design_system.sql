-- +goose Up
-- design-system-catalog: every App carries `design_system_ref` (defaulting to
-- the `ds-forge-default` alias) and an optional `design_system_overrides` map
-- whose keys are component primitives and values are `design_system_ref`s.
--
-- The 0008 migration shipped `design_system_ref` as a nullable text column;
-- backfill is handled by the migration job (see openspec section 8) which
-- sets every existing row to `ds-forge-default` before this migration runs.
-- This script:
--   * Adds the `design_system_overrides` jsonb column (default `{}`)
--   * Sets the default of `design_system_ref` to `ds-forge-default`
--   * Backfills any remaining NULLs to `ds-forge-default`
--   * Flips `design_system_ref` to NOT NULL
--   * Records open swap PRs in a side-table so the API can enforce the
--     one-open-PR-per-App rule and auto-close stale PRs.

ALTER TABLE application
  ADD COLUMN IF NOT EXISTS design_system_overrides jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE application
  ALTER COLUMN design_system_ref SET DEFAULT 'ds-forge-default';

UPDATE application
  SET design_system_ref = 'ds-forge-default'
  WHERE design_system_ref IS NULL OR design_system_ref = '';

ALTER TABLE application
  ALTER COLUMN design_system_ref SET NOT NULL;

CREATE TABLE IF NOT EXISTS application_design_system_pr (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  app_id          uuid        NOT NULL REFERENCES application(id) ON DELETE CASCADE,
  workspace_id    uuid        NOT NULL,
  tenant_id       uuid        NOT NULL,
  target_ref      text        NOT NULL,
  reason          text        NOT NULL DEFAULT '',
  pr_url          text        NOT NULL,
  pr_status       text        NOT NULL DEFAULT 'open'
                    CHECK (pr_status IN ('open','superseded','merged','closed')),
  opened_by       text        NOT NULL,
  opened_at       timestamptz NOT NULL DEFAULT now(),
  closed_at       timestamptz,
  correlation_id  text
);

-- At most one open swap PR per App.
CREATE UNIQUE INDEX IF NOT EXISTS application_design_system_pr_open_idx
  ON application_design_system_pr (app_id)
  WHERE pr_status = 'open';

-- +goose Down
DROP TABLE IF EXISTS application_design_system_pr;
ALTER TABLE application ALTER COLUMN design_system_ref DROP NOT NULL;
ALTER TABLE application ALTER COLUMN design_system_ref DROP DEFAULT;
ALTER TABLE application DROP COLUMN IF EXISTS design_system_overrides;
