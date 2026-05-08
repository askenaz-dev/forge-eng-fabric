-- +goose Up
CREATE TABLE IF NOT EXISTS asset_deployment (
  id TEXT PRIMARY KEY,
  asset_id TEXT NOT NULL,
  env TEXT NOT NULL,
  revision_id TEXT NOT NULL UNIQUE,
  image_digest TEXT NOT NULL,
  runtime_id TEXT NOT NULL,
  verified_status TEXT NOT NULL,
  signature_verified BOOLEAN NOT NULL DEFAULT FALSE,
  attestation_verified BOOLEAN NOT NULL DEFAULT FALSE,
  openspec_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  pr_sha TEXT,
  actor TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS asset_deployment_asset_env_created_idx
  ON asset_deployment(asset_id, env, created_at DESC);

CREATE OR REPLACE FUNCTION reject_asset_deployment_mutation()
RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'asset deployments are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS asset_deployment_immutable_update ON asset_deployment;
CREATE TRIGGER asset_deployment_immutable_update
  BEFORE UPDATE OR DELETE ON asset_deployment
  FOR EACH ROW EXECUTE FUNCTION reject_asset_deployment_mutation();

-- +goose Down
DROP TRIGGER IF EXISTS asset_deployment_immutable_update ON asset_deployment;
DROP FUNCTION IF EXISTS reject_asset_deployment_mutation();
DROP TABLE IF EXISTS asset_deployment;
