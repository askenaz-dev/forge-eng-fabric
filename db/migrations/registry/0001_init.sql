-- +goose Up
-- Asset registry: assets (immutable per version, lifecycle=proposed only in Phase 0)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE asset (
  pk              bigserial PRIMARY KEY,
  id              text NOT NULL,
  version         text NOT NULL,
  type            text NOT NULL CHECK (type IN ('skill','prompt','mcp','workflow','application','repo_template','eval_dataset','healing_action')),
  name            text NOT NULL,
  description     text,
  workspace_id    uuid NOT NULL,
  tenant_id       uuid NOT NULL,
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
