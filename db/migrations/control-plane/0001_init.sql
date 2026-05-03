-- +goose Up
-- Control plane: tenant / business_unit / workspace / github_installation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE tenant (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name        text NOT NULL UNIQUE,
  created_at  timestamptz NOT NULL DEFAULT now(),
  created_by  text,
  archived_at timestamptz
);

CREATE TABLE business_unit (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenant(id) ON DELETE RESTRICT,
  name        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  created_by  text,
  archived_at timestamptz,
  UNIQUE (tenant_id, name)
);
CREATE INDEX ON business_unit (tenant_id);

CREATE TABLE workspace (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         uuid NOT NULL REFERENCES tenant(id) ON DELETE RESTRICT,
  business_unit_id  uuid NOT NULL REFERENCES business_unit(id) ON DELETE RESTRICT,
  name              text NOT NULL,
  description       text,
  owners            text[] NOT NULL DEFAULT '{}',
  created_at        timestamptz NOT NULL DEFAULT now(),
  created_by        text,
  archived_at       timestamptz,
  UNIQUE (business_unit_id, name)
);
CREATE INDEX ON workspace (tenant_id);
CREATE INDEX ON workspace (business_unit_id);

CREATE TABLE github_installation (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenant(id) ON DELETE RESTRICT,
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  installation_id text NOT NULL,
  github_account  text NOT NULL,
  scopes          text[] NOT NULL DEFAULT '{}',
  connected_at    timestamptz NOT NULL DEFAULT now(),
  connected_by    text,
  UNIQUE (workspace_id, installation_id)
);

-- +goose Down
DROP TABLE IF EXISTS github_installation;
DROP TABLE IF EXISTS workspace;
DROP TABLE IF EXISTS business_unit;
DROP TABLE IF EXISTS tenant;
