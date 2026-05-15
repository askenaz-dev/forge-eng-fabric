-- +goose Up
-- Active registry gateways: registry-side schema for the three-pillar pattern
-- (catalog + how-to + gateway). Adds how_to / active_surface / provenance to
-- assets, plus the side-tables that hold per-Tenant external integration
-- metadata (external MCP endpoints, external A2A agents, artifact-store
-- bindings).
--
-- The NOT NULL precondition on how_to_json and active_surface_json is
-- enforced at lifecycle promotion in services/registry, not at the column
-- level: release N keeps the columns nullable so the backfill described in
-- active-registry-gateways/tasks.md 10.1 can run before the precondition
-- starts rejecting writes (release N+1).

-- 1.1 — how-to / active-surface / external-provenance on `asset`.
ALTER TABLE asset ADD COLUMN IF NOT EXISTS how_to_json         jsonb;
ALTER TABLE asset ADD COLUMN IF NOT EXISTS active_surface_json jsonb;
ALTER TABLE asset ADD COLUMN IF NOT EXISTS external_provenance text NOT NULL DEFAULT 'internal';
ALTER TABLE asset ADD CONSTRAINT asset_external_provenance_check
  CHECK (external_provenance IN ('internal','external'));

-- 1.2 — per-Tenant external MCP endpoint. One row per registered external
-- MCP asset; manifest_hash is the digest captured at registration and
-- re-verified on each promotion and by the daily drift cron.
CREATE TABLE IF NOT EXISTS external_mcp_endpoint (
  asset_id            text        PRIMARY KEY,
  tenant_id           uuid        NOT NULL,
  endpoint_url        text        NOT NULL,
  credential_ref      text        NOT NULL,
  allowlist           text[]      NOT NULL DEFAULT '{}',
  manifest_hash       text,
  manifest_fetched_at timestamptz,
  created_by          text        NOT NULL,
  created_at          timestamptz NOT NULL DEFAULT now()
);

-- 1.3 — per-Tenant external A2A agent. Same shape as external_mcp_endpoint;
-- agent_card_hash captures the A2A agent-card digest at registration.
CREATE TABLE IF NOT EXISTS external_a2a_agent (
  asset_id              text        PRIMARY KEY,
  tenant_id             uuid        NOT NULL,
  endpoint_url          text        NOT NULL,
  credential_ref        text        NOT NULL,
  task_allowlist        text[]      NOT NULL DEFAULT '{}',
  agent_card_hash       text,
  agent_card_fetched_at timestamptz,
  created_by            text        NOT NULL,
  created_at            timestamptz NOT NULL DEFAULT now()
);

-- 1.4 — per-Tenant artifact-store binding. The decision in
-- active-registry-gateways/design.md §3 is one binding per Tenant; revisit
-- if customer feedback demands per-asset-family bindings (tasks.md 11.2).
CREATE TABLE IF NOT EXISTS artifact_store_binding (
  tenant_id   uuid        PRIMARY KEY,
  backend     text        NOT NULL,
  config_json jsonb       NOT NULL DEFAULT '{}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT artifact_store_binding_backend_check
    CHECK (backend IN ('nexus','artifactory','github-packages-private','codeartifact'))
);

-- 1.5 — hot-path indices.
-- Partial index over assets missing active_surface_json; Portal uses this to
-- list assets that still need backfill before the release N+1 precondition
-- starts enforcing.
CREATE INDEX IF NOT EXISTS asset_missing_active_surface_idx
  ON asset (tenant_id, type, lifecycle_state)
  WHERE active_surface_json IS NULL;

CREATE INDEX IF NOT EXISTS external_mcp_endpoint_tenant_asset_idx
  ON external_mcp_endpoint (tenant_id, asset_id);

CREATE INDEX IF NOT EXISTS external_a2a_agent_tenant_asset_idx
  ON external_a2a_agent (tenant_id, asset_id);

-- +goose Down
DROP INDEX IF EXISTS external_a2a_agent_tenant_asset_idx;
DROP INDEX IF EXISTS external_mcp_endpoint_tenant_asset_idx;
DROP INDEX IF EXISTS asset_missing_active_surface_idx;

DROP TABLE IF EXISTS artifact_store_binding;
DROP TABLE IF EXISTS external_a2a_agent;
DROP TABLE IF EXISTS external_mcp_endpoint;

ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_external_provenance_check;
ALTER TABLE asset DROP COLUMN IF EXISTS external_provenance;
ALTER TABLE asset DROP COLUMN IF EXISTS active_surface_json;
ALTER TABLE asset DROP COLUMN IF EXISTS how_to_json;
