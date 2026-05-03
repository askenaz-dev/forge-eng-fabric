-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE delegated_permission (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  subject           text NOT NULL,
  scope_kind        text NOT NULL,
  scope_id          text NOT NULL,
  action_class      text NOT NULL,
  max_criticality   text NOT NULL,
  expires_at        timestamptz NOT NULL,
  justification     text NOT NULL,
  requester         text NOT NULL,
  approver          text NOT NULL,
  status            text NOT NULL CHECK (status IN ('active', 'revoked', 'expired')),
  openfga_tuple     jsonb NOT NULL,
  audit_history     jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX delegated_permission_lookup_idx ON delegated_permission(subject, scope_kind, scope_id, action_class, status);
CREATE INDEX delegated_permission_expiration_idx ON delegated_permission(expires_at) WHERE status = 'active';

-- +goose Down
DROP TABLE IF EXISTS delegated_permission;
