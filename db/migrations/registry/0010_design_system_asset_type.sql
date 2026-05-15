-- +goose Up
-- design-system-catalog: add `design_system` as a Registry asset type.
--   * relax the asset.type CHECK constraint to admit `design_system`
--   * carry the design-system manifest body alongside the asset row
--   * mark the four built-in catalog templates so the sanity validator
--     can be bypassed atomically with an audit-recorded reason

ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_type_check;
ALTER TABLE asset ADD CONSTRAINT asset_type_check
  CHECK (type IN ('mcp','skill','agent','workflow','prompt_template','application','repo_template','design_system'));

ALTER TABLE asset
  ADD COLUMN IF NOT EXISTS design_system_manifest jsonb,
  ADD COLUMN IF NOT EXISTS built_in_template boolean NOT NULL DEFAULT false;

-- A non-built-in design_system asset MUST have a manifest. Built-in templates
-- ship pre-validated and skip the structural CHECK so the seed migration can
-- insert the body in the same transaction as the row.
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_design_system_manifest_check;
ALTER TABLE asset ADD CONSTRAINT asset_design_system_manifest_check
  CHECK (
    type <> 'design_system'
    OR built_in_template = true
    OR design_system_manifest IS NOT NULL
  );

CREATE INDEX IF NOT EXISTS asset_design_system_idx
  ON asset (type)
  WHERE type = 'design_system';

-- Persistent aliases for design-system references (`ds-forge-default`, ...).
-- Resolved at runtime by registry reads and at build-time by the portal merger.
CREATE TABLE IF NOT EXISTS design_system_alias (
  alias        text        PRIMARY KEY,
  asset_id     text        NOT NULL,
  retargeted_at timestamptz NOT NULL DEFAULT now(),
  retargeted_by text        NOT NULL DEFAULT 'system:forge-platform'
);

-- +goose Down
DROP TABLE IF EXISTS design_system_alias;
DROP INDEX IF EXISTS asset_design_system_idx;
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_design_system_manifest_check;
ALTER TABLE asset DROP COLUMN IF EXISTS built_in_template;
ALTER TABLE asset DROP COLUMN IF EXISTS design_system_manifest;
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_type_check;
ALTER TABLE asset ADD CONSTRAINT asset_type_check
  CHECK (type IN ('mcp','skill','agent','workflow','prompt_template','application','repo_template'));
