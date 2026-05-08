-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE runtime (
  id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id             uuid NOT NULL,
  tenant_id                uuid NOT NULL,
  type                     text NOT NULL CHECK (type IN ('gke','cloudrun','minikube')),
  mode                     text NOT NULL CHECK (mode IN ('byo','provisioned')),
  visibility               text NOT NULL DEFAULT 'workspace' CHECK (visibility IN ('workspace','tenant')),
  name                     text NOT NULL,
  region                   text,
  gke_mode                 text CHECK (gke_mode IN ('standard','autopilot') OR gke_mode IS NULL),
  project_id               text,
  cluster_name             text,
  endpoint                 text,
  service_account_email    text,
  namespace                text,
  credential_kms_key_ref   text,
  credential_cipher_b64    text,
  labels                   jsonb NOT NULL DEFAULT '{}'::jsonb,
  capabilities             jsonb NOT NULL DEFAULT '{}'::jsonb,
  status                   text NOT NULL DEFAULT 'registered',
  revoked                  boolean NOT NULL DEFAULT false,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),
  CHECK (
    -- BYO requires either an encrypted credential or a service account email reference;
    -- plaintext credentials are forbidden.
    (mode = 'provisioned')
    OR (mode = 'byo' AND (credential_cipher_b64 IS NOT NULL AND credential_kms_key_ref IS NOT NULL))
  )
);
CREATE INDEX runtime_workspace_idx ON runtime(workspace_id);
CREATE INDEX runtime_tenant_idx ON runtime(tenant_id);

CREATE TABLE runtime_preflight_result (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  runtime_id   uuid NOT NULL REFERENCES runtime(id) ON DELETE CASCADE,
  outcome      text NOT NULL CHECK (outcome IN ('success','failed')),
  reason       text,
  checks       jsonb NOT NULL DEFAULT '[]'::jsonb,
  metadata     jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at   timestamptz NOT NULL,
  ended_at     timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX runtime_preflight_result_runtime_idx ON runtime_preflight_result(runtime_id, created_at DESC);

CREATE TABLE tenant_iac_state_backend (
  tenant_id    uuid PRIMARY KEY,
  bucket       text NOT NULL,
  kms_key_ref  text,
  versioning   boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS tenant_iac_state_backend;
DROP TABLE IF EXISTS runtime_preflight_result;
DROP TABLE IF EXISTS runtime;
