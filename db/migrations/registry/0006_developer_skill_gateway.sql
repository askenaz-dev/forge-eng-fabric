-- +goose Up
-- Phase: developer skill gateway. Add distribution metadata to `asset`, plus
-- the gateway-side tables (packages, PATs, install records).

-- 1.1 — distribution block on asset
ALTER TABLE asset ADD COLUMN IF NOT EXISTS distribution_gateway_published boolean NOT NULL DEFAULT false;
ALTER TABLE asset ADD COLUMN IF NOT EXISTS distribution_gateway_channel    text    NOT NULL DEFAULT 'stable';
ALTER TABLE asset ADD COLUMN IF NOT EXISTS distribution_package_digest     text;
ALTER TABLE asset ADD COLUMN IF NOT EXISTS distribution_package_signed_at  timestamptz;
ALTER TABLE asset ADD COLUMN IF NOT EXISTS distribution_deprecation_pointer text;
ALTER TABLE asset ADD CONSTRAINT asset_distribution_channel_check
  CHECK (distribution_gateway_channel IN ('stable','beta'));

-- 1.2 — packaged Agent Skills bundles produced by the platform packager.
-- Bundles are content-addressed; (asset_id,version) -> single canonical digest.
CREATE TABLE IF NOT EXISTS asset_package (
  asset_id        text        NOT NULL,
  version         text        NOT NULL,
  digest          text        NOT NULL,
  signature_id    text        NOT NULL,
  attestation_id  text        NOT NULL,
  bytes_uri       text        NOT NULL,
  size_bytes      bigint      NOT NULL CHECK (size_bytes >= 0),
  channel         text        NOT NULL DEFAULT 'stable' CHECK (channel IN ('stable','beta')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (asset_id, version)
);

CREATE UNIQUE INDEX IF NOT EXISTS asset_package_digest_idx ON asset_package(digest);

CREATE OR REPLACE FUNCTION reject_asset_package_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'asset_package rows are immutable; publish a new asset version to rotate';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS asset_package_immutable ON asset_package;
CREATE TRIGGER asset_package_immutable
  BEFORE UPDATE OR DELETE ON asset_package
  FOR EACH ROW EXECUTE FUNCTION reject_asset_package_mutation();

-- 1.3 — developer personal access tokens for the gateway.
-- `hashed_secret` is argon2id(plaintext); plaintext is shown to the developer
-- only at issuance and never persisted.
CREATE TABLE IF NOT EXISTS gateway_token (
  id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id            uuid        NOT NULL,
  developer_sub        text        NOT NULL,
  assume_workspace_id  uuid        NOT NULL,
  scopes               text[]      NOT NULL,
  asset_allowlist      text[]      NOT NULL DEFAULT '{}',
  hashed_secret        text        NOT NULL,
  created_by           text        NOT NULL,
  created_at           timestamptz NOT NULL DEFAULT now(),
  last_used_at         timestamptz,
  expires_at           timestamptz NOT NULL,
  revoked_at           timestamptz,
  CONSTRAINT gateway_token_scope_subset CHECK (
    scopes <@ ARRAY['gateway.read','gateway.install','gateway.invoke']::text[]
  ),
  CONSTRAINT gateway_token_lifetime CHECK (expires_at > created_at)
);

-- 1.4 — install records keyed by (developer, asset, client). Idempotent on
-- reinstall: same key, updated version + last_seen_at.
CREATE TABLE IF NOT EXISTS gateway_install (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  developer_sub   text        NOT NULL,
  tenant_id       uuid        NOT NULL,
  asset_id        text        NOT NULL,
  version         text        NOT NULL,
  client          text        NOT NULL,
  installed_at    timestamptz NOT NULL DEFAULT now(),
  last_seen_at    timestamptz NOT NULL DEFAULT now(),
  removed_at      timestamptz,
  UNIQUE (developer_sub, asset_id, client)
);

-- 1.5 — hot-path indices.
CREATE INDEX IF NOT EXISTS asset_distribution_publication_idx
  ON asset(tenant_id, distribution_gateway_published, type)
  WHERE distribution_gateway_published = true;

CREATE INDEX IF NOT EXISTS gateway_token_hashed_secret_idx
  ON gateway_token(hashed_secret)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS gateway_token_developer_idx
  ON gateway_token(developer_sub, tenant_id);

CREATE INDEX IF NOT EXISTS gateway_install_developer_idx
  ON gateway_install(developer_sub, asset_id, client);

CREATE INDEX IF NOT EXISTS gateway_install_active_idx
  ON gateway_install(asset_id, last_seen_at DESC)
  WHERE removed_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS gateway_install_active_idx;
DROP INDEX IF EXISTS gateway_install_developer_idx;
DROP INDEX IF EXISTS gateway_token_developer_idx;
DROP INDEX IF EXISTS gateway_token_hashed_secret_idx;
DROP INDEX IF EXISTS asset_distribution_publication_idx;

DROP TABLE IF EXISTS gateway_install;
DROP TABLE IF EXISTS gateway_token;

DROP TRIGGER IF EXISTS asset_package_immutable ON asset_package;
DROP FUNCTION IF EXISTS reject_asset_package_mutation();
DROP INDEX IF EXISTS asset_package_digest_idx;
DROP TABLE IF EXISTS asset_package;

ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_distribution_channel_check;
ALTER TABLE asset DROP COLUMN IF EXISTS distribution_deprecation_pointer;
ALTER TABLE asset DROP COLUMN IF EXISTS distribution_package_signed_at;
ALTER TABLE asset DROP COLUMN IF EXISTS distribution_package_digest;
ALTER TABLE asset DROP COLUMN IF EXISTS distribution_gateway_channel;
ALTER TABLE asset DROP COLUMN IF EXISTS distribution_gateway_published;
