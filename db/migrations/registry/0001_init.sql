-- +goose Up
-- Asset registry: assets (immutable per version, lifecycle=proposed only in Phase 0)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE asset (
  pk              bigserial PRIMARY KEY,
  id              text NOT NULL,
  version         text NOT NULL,
  type            text NOT NULL CHECK (type IN ('mcp','skill','agent','workflow','prompt_template')),
  name            text NOT NULL,
  description     text,
  owner_team      text NOT NULL,
  inputs_schema   jsonb NOT NULL DEFAULT '{}'::jsonb,
  outputs_schema  jsonb NOT NULL DEFAULT '{}'::jsonb,
  workspace_id    uuid NOT NULL,
  tenant_id       uuid NOT NULL,
  visibility      text NOT NULL DEFAULT 'workspace' CHECK (visibility IN ('workspace','tenant')),
  lifecycle_state text NOT NULL DEFAULT 'proposed' CHECK (lifecycle_state = 'proposed'),
  trust_level     text,
  owners          text[] NOT NULL DEFAULT '{}',
  metadata        jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at      timestamptz NOT NULL DEFAULT now(),
  created_by      text,
  UNIQUE (id, version)
);
CREATE INDEX ON asset (workspace_id);
CREATE INDEX ON asset (tenant_id);
CREATE INDEX ON asset (type);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION asset_immutable_version() RETURNS trigger AS $$
BEGIN
  -- A specific (id, version) row may not be modified once written.
  RAISE EXCEPTION 'asset versions are immutable (id=%, version=%)', OLD.id, OLD.version;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER asset_no_update BEFORE UPDATE ON asset
  FOR EACH ROW EXECUTE FUNCTION asset_immutable_version();
CREATE TRIGGER asset_no_delete BEFORE DELETE ON asset
  FOR EACH ROW EXECUTE FUNCTION asset_immutable_version();

-- +goose Down
DROP TRIGGER IF EXISTS asset_no_delete ON asset;
DROP TRIGGER IF EXISTS asset_no_update ON asset;
DROP FUNCTION IF EXISTS asset_immutable_version();
DROP TABLE IF EXISTS asset;
