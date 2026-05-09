-- +goose Up
-- platform-gaps-closure 7.1: monthly partition rotation + retention metadata.
-- Adds a helper that ensures the next N months exist, plus tables for legal
-- holds and retention runs. The append-only triggers from 0001 already block
-- direct DELETE; the retention job uses ALTER TABLE ... DETACH PARTITION which
-- bypasses row triggers and converts the partition into a standalone table that
-- can be dropped after archival.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_audit_event_partitions(months_ahead int) RETURNS void AS $$
DECLARE
  i int;
  m date;
BEGIN
  FOR i IN 0..months_ahead LOOP
    m := (date_trunc('month', now()) + (i || ' months')::interval)::date;
    PERFORM create_audit_event_month_partition(m);
  END LOOP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Convenience: list partitions older than `cutoff`. The retention job calls
-- this to discover candidates for archival.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION list_audit_partitions_older_than(cutoff timestamptz)
RETURNS TABLE(partition_name text, range_start timestamptz, range_end timestamptz) AS $$
BEGIN
  RETURN QUERY
  SELECT child.relname::text,
         (regexp_match(pg_get_expr(child.relpartbound, child.oid),
                       'FROM \(''(.+?)''\) TO \(''(.+?)''\)'))[1]::timestamptz,
         (regexp_match(pg_get_expr(child.relpartbound, child.oid),
                       'FROM \(''(.+?)''\) TO \(''(.+?)''\)'))[2]::timestamptz
  FROM pg_inherits
       JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
       JOIN pg_class child  ON pg_inherits.inhrelid  = child.oid
  WHERE parent.relname = 'audit_event'
    AND child.relname  <> 'audit_event_default'
    AND (regexp_match(pg_get_expr(child.relpartbound, child.oid),
                      'FROM \(''(.+?)''\) TO \(''(.+?)''\)'))[2]::timestamptz <= cutoff;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS retention_legal_hold (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope         text NOT NULL CHECK (scope IN ('tenant','business_unit','workspace')),
  scope_id      text NOT NULL,
  data_type     text NOT NULL,
  selector      jsonb NOT NULL,
  reason        text NOT NULL,
  approver      text NOT NULL,
  set_at        timestamptz NOT NULL DEFAULT now(),
  released_at   timestamptz,
  released_by   text,
  expires_at    timestamptz
);
CREATE INDEX IF NOT EXISTS retention_legal_hold_scope_idx
  ON retention_legal_hold (scope, scope_id, data_type);
CREATE INDEX IF NOT EXISTS retention_legal_hold_active_idx
  ON retention_legal_hold (data_type) WHERE released_at IS NULL;

CREATE TABLE IF NOT EXISTS retention_run (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  data_type       text NOT NULL,
  partition_name  text,
  scope_id        text,
  classification  text,
  outcome         text NOT NULL CHECK (outcome IN ('dryrun_logged','archived','skipped_legal_hold','failed')),
  enforce         boolean NOT NULL DEFAULT false,
  records_count   bigint,
  archive_uri     text,
  started_at      timestamptz NOT NULL DEFAULT now(),
  ended_at        timestamptz,
  notes           text
);
CREATE INDEX IF NOT EXISTS retention_run_data_type_idx ON retention_run (data_type, started_at DESC);

-- +goose Down
DROP TABLE IF EXISTS retention_run;
DROP TABLE IF EXISTS retention_legal_hold;
DROP FUNCTION IF EXISTS list_audit_partitions_older_than(timestamptz);
DROP FUNCTION IF EXISTS ensure_audit_event_partitions(int);
