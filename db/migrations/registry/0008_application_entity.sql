-- +goose Up
-- Phase 5: introduce Application ("App") as a first-class entity sitting between
-- Workspace and OpenSpec. See openspec/changes/app-first-class-entity for the
-- full rationale; rolled out in three steps: M1 add nullable FKs, M2 backfill,
-- M3 flip to NOT NULL (handled by db/migrations/registry/0009_app_id_not_null.sql
-- once the per-workspace backfill sentinels are set).

-- 1.1 — application aggregate.
CREATE TABLE IF NOT EXISTS application (
  id                    uuid        PRIMARY KEY,
  slug                  text        NOT NULL,
  name                  text        NOT NULL,
  description           text        NOT NULL DEFAULT '',
  workspace_id          uuid        NOT NULL,
  tenant_id             uuid        NOT NULL,
  lifecycle_state       text        NOT NULL DEFAULT 'active'
                          CHECK (lifecycle_state IN ('active','archived','deleted')),
  design_system_ref     text,
  default_environments  jsonb       NOT NULL DEFAULT '[]'::jsonb,
  repo_links            jsonb       NOT NULL DEFAULT '[]'::jsonb,
  runtime_links         jsonb       NOT NULL DEFAULT '[]'::jsonb,
  owners                jsonb       NOT NULL DEFAULT '[]'::jsonb,
  system_managed        boolean     NOT NULL DEFAULT false,
  created_at            timestamptz NOT NULL DEFAULT now(),
  created_by            text        NOT NULL,
  updated_at            timestamptz NOT NULL DEFAULT now(),
  updated_by            text,
  archived_at           timestamptz,
  deleted_at            timestamptz,
  CONSTRAINT application_slug_format
    CHECK (slug ~ '^[a-z0-9][a-z0-9_-]{0,62}$'),
  CONSTRAINT application_owners_min
    CHECK (jsonb_typeof(owners) = 'array' AND jsonb_array_length(owners) >= 1)
);

CREATE UNIQUE INDEX IF NOT EXISTS application_workspace_slug_uidx
  ON application (workspace_id, slug);
CREATE INDEX IF NOT EXISTS application_workspace_idx
  ON application (workspace_id);
CREATE INDEX IF NOT EXISTS application_tenant_idx
  ON application (tenant_id);
CREATE INDEX IF NOT EXISTS application_lifecycle_idx
  ON application (workspace_id, lifecycle_state);

-- 1.2 (partial) — wire `app_id` on `asset` and `asset_deployment`. The remaining
-- tables live in other migration directories (openspec/0002, app-onboarding/0003,
-- runtime-registry/0002) but are listed here for trace-ability:
--   - openspec_index.app_id        -> db/migrations/openspec/0002_app_id.sql
--   - app_onboarding_request.app_id-> db/migrations/app-onboarding/0003_app_id.sql
--   - runtime.app_id               -> db/migrations/runtime-registry/0002_app_id.sql

ALTER TABLE asset
  ADD COLUMN IF NOT EXISTS app_id uuid REFERENCES application(id);
CREATE INDEX IF NOT EXISTS asset_app_id_idx ON asset (app_id);

ALTER TABLE asset_deployment
  ADD COLUMN IF NOT EXISTS app_id uuid REFERENCES application(id);
CREATE INDEX IF NOT EXISTS asset_deployment_app_id_idx ON asset_deployment (app_id);

-- 1.3 — workspace/app-workspace coherence is enforced in the app layer for now
-- (Application#3.6, OpenSpec backbone#5.2). The DB-level CHECK constraint is
-- added in 0009_app_id_not_null.sql once backfill has guaranteed no NULL FKs.

-- 1.4 — application_audit, partitioned monthly by created_at. Records every
-- lifecycle transition; rows are immutable.
CREATE TABLE IF NOT EXISTS application_audit (
  id              uuid        NOT NULL DEFAULT gen_random_uuid(),
  app_id          uuid        NOT NULL,
  workspace_id    uuid        NOT NULL,
  tenant_id       uuid        NOT NULL,
  action          text        NOT NULL
                    CHECK (action IN ('created','updated','archived','restored','deleted','reparent_in','reparent_out','migration_confirmation','spec_purged')),
  actor           text        NOT NULL,
  correlation_id  text,
  reason          text,
  before          jsonb,
  after           jsonb,
  evidence        jsonb       NOT NULL DEFAULT '{}'::jsonb,
  created_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Seed the first three monthly partitions (current + 2 next months). Production
-- runbooks roll forward partitions monthly; this is enough to bootstrap.
CREATE TABLE IF NOT EXISTS application_audit_p2026_05
  PARTITION OF application_audit FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS application_audit_p2026_06
  PARTITION OF application_audit FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS application_audit_p2026_07
  PARTITION OF application_audit FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE INDEX IF NOT EXISTS application_audit_app_idx
  ON application_audit (app_id, created_at DESC);
CREATE INDEX IF NOT EXISTS application_audit_workspace_idx
  ON application_audit (workspace_id, created_at DESC);

-- Audit rows must be immutable.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reject_application_audit_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'application_audit rows are immutable';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS application_audit_immutable ON application_audit;
CREATE TRIGGER application_audit_immutable
  BEFORE UPDATE OR DELETE ON application_audit
  FOR EACH ROW EXECUTE FUNCTION reject_application_audit_mutation();

-- Per-workspace migration sentinel used by 0009_app_id_not_null.sql to gate
-- the NOT NULL flip. The migration job (cmd/spec-app-migration execute) inserts
-- one row per workspace once it has verified there are zero NULL app_id rows
-- across openspec_index, app_onboarding_request, runtime and asset for that
-- workspace.
CREATE TABLE IF NOT EXISTS application_backfill_sentinel (
  workspace_id  uuid        PRIMARY KEY,
  set_at        timestamptz NOT NULL DEFAULT now(),
  set_by        text        NOT NULL,
  notes         text
);

-- +goose Down
DROP TABLE IF EXISTS application_backfill_sentinel;
DROP TRIGGER IF EXISTS application_audit_immutable ON application_audit;
DROP FUNCTION IF EXISTS reject_application_audit_mutation();
DROP TABLE IF EXISTS application_audit_p2026_07;
DROP TABLE IF EXISTS application_audit_p2026_06;
DROP TABLE IF EXISTS application_audit_p2026_05;
DROP TABLE IF EXISTS application_audit;

DROP INDEX IF EXISTS asset_deployment_app_id_idx;
ALTER TABLE asset_deployment DROP COLUMN IF EXISTS app_id;

DROP INDEX IF EXISTS asset_app_id_idx;
ALTER TABLE asset DROP COLUMN IF EXISTS app_id;

DROP INDEX IF EXISTS application_lifecycle_idx;
DROP INDEX IF EXISTS application_tenant_idx;
DROP INDEX IF EXISTS application_workspace_idx;
DROP INDEX IF EXISTS application_workspace_slug_uidx;
DROP TABLE IF EXISTS application;
