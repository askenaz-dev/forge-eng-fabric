-- +goose Up
-- Audit: append-only events with per-tenant prev_hash chain (D0.5).
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE audit_event (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL,
  workspace_id    uuid,
  actor           text NOT NULL,
  action          text NOT NULL,
  resource        text NOT NULL,
  outcome         text NOT NULL CHECK (outcome IN ('success','denied','error')),
  details         jsonb NOT NULL DEFAULT '{}'::jsonb,
  correlation_id  text,
  prev_hash       text NOT NULL,
  hash            text NOT NULL,
  occurred_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON audit_event (tenant_id, occurred_at DESC);
CREATE INDEX ON audit_event (workspace_id);
CREATE INDEX ON audit_event (correlation_id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_event_chain() RETURNS trigger AS $$
DECLARE
  last_hash text;
  payload   text;
BEGIN
  SELECT hash INTO last_hash
    FROM audit_event
   WHERE tenant_id = NEW.tenant_id
   ORDER BY occurred_at DESC, id DESC
   LIMIT 1;

  IF last_hash IS NULL THEN
    last_hash := repeat('0', 64);
  END IF;

  NEW.prev_hash := last_hash;
  payload := concat_ws('|',
    NEW.tenant_id::text,
    coalesce(NEW.workspace_id::text,''),
    NEW.actor, NEW.action, NEW.resource, NEW.outcome,
    NEW.details::text,
    coalesce(NEW.correlation_id,''),
    NEW.occurred_at::text,
    last_hash);
  NEW.hash := encode(digest(payload, 'sha256'), 'hex');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_event_chain_trg BEFORE INSERT ON audit_event
  FOR EACH ROW EXECUTE FUNCTION audit_event_chain();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_event_no_modify() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'audit_event is append-only (id=%)', coalesce(OLD.id::text, '?');
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_event_no_update BEFORE UPDATE ON audit_event
  FOR EACH ROW EXECUTE FUNCTION audit_event_no_modify();
CREATE TRIGGER audit_event_no_delete BEFORE DELETE ON audit_event
  FOR EACH ROW EXECUTE FUNCTION audit_event_no_modify();

-- +goose Down
DROP TRIGGER IF EXISTS audit_event_no_delete ON audit_event;
DROP TRIGGER IF EXISTS audit_event_no_update ON audit_event;
DROP TRIGGER IF EXISTS audit_event_chain_trg ON audit_event;
DROP FUNCTION IF EXISTS audit_event_no_modify();
DROP FUNCTION IF EXISTS audit_event_chain();
DROP TABLE IF EXISTS audit_event;
