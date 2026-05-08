CREATE TABLE IF NOT EXISTS deployment (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL UNIQUE,
  workspace_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  env TEXT NOT NULL,
  criticality TEXT,
  data_classification TEXT,
  runtime_id TEXT NOT NULL,
  image TEXT NOT NULL,
  image_digest TEXT NOT NULL,
  manifest_sha TEXT,
  revision_id TEXT NOT NULL UNIQUE,
  source_revision_id TEXT,
  rollback_of TEXT REFERENCES deployment(id),
  openspec_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  pr_sha TEXT,
  strategy TEXT NOT NULL DEFAULT 'rolling',
  canary_percent INTEGER,
  rollback_plan TEXT,
  auto_rollback BOOLEAN NOT NULL DEFAULT FALSE,
  status TEXT NOT NULL,
  status_reason TEXT,
  verified_signature BOOLEAN NOT NULL DEFAULT FALSE,
  verified_attestation BOOLEAN NOT NULL DEFAULT FALSE,
  correlation_id TEXT NOT NULL,
  actor TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS deployment_asset_env_created_idx
  ON deployment(asset_id, env, created_at DESC);

CREATE TABLE IF NOT EXISTS deployment_event (
  id TEXT PRIMARY KEY,
  deployment_id TEXT NOT NULL REFERENCES deployment(id),
  stage TEXT NOT NULL,
  outcome TEXT NOT NULL,
  reason TEXT,
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ NOT NULL,
  duration_ms BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS deployment_event_deployment_started_idx
  ON deployment_event(deployment_id, started_at);

CREATE TABLE IF NOT EXISTS deployment_policy_eval (
  id TEXT PRIMARY KEY,
  deployment_id TEXT NOT NULL REFERENCES deployment(id),
  policy_id TEXT NOT NULL,
  outcome TEXT NOT NULL,
  reason TEXT,
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  evaluated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS image_verification_result (
  id TEXT PRIMARY KEY,
  deployment_id TEXT NOT NULL REFERENCES deployment(id),
  outcome TEXT NOT NULL,
  reason TEXT,
  identity TEXT,
  digest TEXT,
  signature_verified BOOLEAN NOT NULL DEFAULT FALSE,
  attestation_verified BOOLEAN NOT NULL DEFAULT FALSE,
  override_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rollback_record (
  id TEXT PRIMARY KEY,
  deployment_id TEXT NOT NULL REFERENCES deployment(id),
  source_revision_id TEXT NOT NULL,
  restored_revision_id TEXT NOT NULL,
  reason TEXT NOT NULL,
  trigger TEXT NOT NULL,
  approved BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE OR REPLACE FUNCTION reject_deployment_revision_mutation()
RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'deployment revisions are immutable';
  END IF;

  IF OLD.revision_id <> NEW.revision_id
    OR OLD.asset_id <> NEW.asset_id
    OR OLD.env <> NEW.env
    OR OLD.image_digest <> NEW.image_digest
    OR COALESCE(OLD.manifest_sha, '') <> COALESCE(NEW.manifest_sha, '') THEN
    RAISE EXCEPTION 'deployment revision fields are immutable';
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS deployment_revision_immutable ON deployment;
CREATE TRIGGER deployment_revision_immutable
  BEFORE UPDATE OR DELETE ON deployment
  FOR EACH ROW EXECUTE FUNCTION reject_deployment_revision_mutation();
