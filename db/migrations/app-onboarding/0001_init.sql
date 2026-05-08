-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE repo_template (
  id              text PRIMARY KEY,
  category        text NOT NULL,
  description     text,
  created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE repo_template_version (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id     text NOT NULL REFERENCES repo_template(id),
  version         text NOT NULL,
  manifest        jsonb NOT NULL,
  trust_level     text NOT NULL DEFAULT 'T1',
  lifecycle_state text NOT NULL DEFAULT 'proposed',
  signed          boolean NOT NULL DEFAULT false,
  source_url      text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (template_id, version)
);

CREATE TABLE app_onboarding_request (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    uuid NOT NULL,
  tenant_id       uuid NOT NULL,
  repo_org        text NOT NULL,
  repo_name       text NOT NULL,
  template_id     text NOT NULL,
  template_version text NOT NULL,
  parameters      jsonb NOT NULL DEFAULT '{}'::jsonb,
  criticality     text NOT NULL DEFAULT 'medium',
  data_classification text NOT NULL DEFAULT 'internal',
  owners          text[] NOT NULL DEFAULT '{}',
  status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','pending_approval','running','completed','failed')),
  status_reason   text,
  asset_id        text,
  correlation_id  text NOT NULL,
  requested_by    text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  completed_at    timestamptz,
  UNIQUE (workspace_id, repo_name)
);

CREATE INDEX app_onboarding_request_status_idx ON app_onboarding_request(status);
CREATE INDEX app_onboarding_request_workspace_idx ON app_onboarding_request(workspace_id);

CREATE TABLE app_onboarding_event (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id      uuid NOT NULL REFERENCES app_onboarding_request(id) ON DELETE CASCADE,
  stage           text NOT NULL,
  outcome         text NOT NULL CHECK (outcome IN ('started','completed','failed','warn')),
  message         text,
  payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
  duration_ms     bigint,
  created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX app_onboarding_event_request_idx ON app_onboarding_event(request_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS app_onboarding_event;
DROP TABLE IF EXISTS app_onboarding_request;
DROP TABLE IF EXISTS repo_template_version;
DROP TABLE IF EXISTS repo_template;
