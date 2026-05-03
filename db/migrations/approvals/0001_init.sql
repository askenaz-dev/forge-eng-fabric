-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE approval_request (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  principal           text NOT NULL,
  action              text NOT NULL,
  workspace_id        uuid NOT NULL,
  openspec_id         text,
  target              jsonb NOT NULL DEFAULT '{}'::jsonb,
  rationale           text NOT NULL,
  required_approvers  text[] NOT NULL DEFAULT '{}',
  criticality         text NOT NULL,
  correlation_id      text NOT NULL,
  status              text NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
  requested_at        timestamptz NOT NULL DEFAULT now(),
  expires_at          timestamptz NOT NULL,
  decided_by          text,
  decided_at          timestamptz,
  decision_comment    text
);

CREATE INDEX approval_request_pending_idx ON approval_request(expires_at) WHERE status = 'pending';
CREATE INDEX approval_request_workspace_idx ON approval_request(workspace_id, status);

-- +goose Down
DROP TABLE IF EXISTS approval_request;
