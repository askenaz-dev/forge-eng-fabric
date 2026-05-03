-- +goose Up
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_lifecycle_state_check;
ALTER TABLE asset ADD CONSTRAINT asset_lifecycle_state_check
  CHECK (lifecycle_state IN ('proposed','in_review','approved','deprecated','retired'));

ALTER TABLE asset ALTER COLUMN trust_level SET DEFAULT 'T0';
UPDATE asset SET trust_level = 'T0' WHERE trust_level IS NULL;
ALTER TABLE asset ALTER COLUMN trust_level SET NOT NULL;
ALTER TABLE asset ADD CONSTRAINT asset_trust_level_check CHECK (trust_level IN ('T0','T1','T2','T3','T4','T5'));

ALTER TABLE asset ADD COLUMN IF NOT EXISTS eval_scores jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE asset ADD COLUMN IF NOT EXISTS recommended_replacement text;

DROP TRIGGER IF EXISTS asset_no_update ON asset;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION asset_phase1_mutable_fields_only() RETURNS trigger AS $$
BEGIN
  IF OLD.id <> NEW.id OR OLD.version <> NEW.version OR OLD.type <> NEW.type OR OLD.name <> NEW.name
     OR OLD.workspace_id <> NEW.workspace_id OR OLD.tenant_id <> NEW.tenant_id THEN
    RAISE EXCEPTION 'asset identity fields are immutable (id=%, version=%)', OLD.id, OLD.version;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER asset_phase1_update BEFORE UPDATE ON asset
  FOR EACH ROW EXECUTE FUNCTION asset_phase1_mutable_fields_only();

CREATE TABLE asset_lifecycle_event (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id         text NOT NULL,
  version          text NOT NULL,
  from_state       text NOT NULL,
  to_state         text NOT NULL,
  trust_level      text NOT NULL,
  eval_scores      jsonb NOT NULL DEFAULT '{}'::jsonb,
  actor            text,
  created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX asset_lifecycle_event_asset_idx ON asset_lifecycle_event(asset_id, version, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS asset_lifecycle_event;
DROP TRIGGER IF EXISTS asset_phase1_update ON asset;
DROP FUNCTION IF EXISTS asset_phase1_mutable_fields_only();
CREATE TRIGGER asset_no_update BEFORE UPDATE ON asset
  FOR EACH ROW EXECUTE FUNCTION asset_immutable_version();
ALTER TABLE asset DROP COLUMN IF EXISTS recommended_replacement;
ALTER TABLE asset DROP COLUMN IF EXISTS eval_scores;
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_trust_level_check;
ALTER TABLE asset ALTER COLUMN trust_level DROP NOT NULL;
ALTER TABLE asset ALTER COLUMN trust_level DROP DEFAULT;
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_lifecycle_state_check;
ALTER TABLE asset ADD CONSTRAINT asset_lifecycle_state_check CHECK (lifecycle_state = 'proposed');
