-- +goose Up
-- Symptom-triager persistence layer.
-- Tables are created empty at iter 1; rows are populated from iter 2 onward.

CREATE TABLE noise_rule (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid,
  fingerprint     text NOT NULL,
  description     text NOT NULL,
  proposed_by     text NOT NULL,
  proposed_at     timestamptz NOT NULL DEFAULT now(),
  approved_by     text,
  approved_at     timestamptz,
  promoted_at     timestamptz,
  revoked_at      timestamptz,
  expires_at      timestamptz,
  pr_url          text,
  evidence_sample_ids text[],
  status          text NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'active', 'promoted', 'revoked'))
);

CREATE INDEX noise_rule_fingerprint_idx ON noise_rule(fingerprint) WHERE status IN ('active', 'promoted');
CREATE INDEX noise_rule_tenant_idx ON noise_rule(tenant_id) WHERE tenant_id IS NOT NULL;

CREATE TABLE circuit_breaker_state (
  fingerprint           text PRIMARY KEY,
  opened_at             timestamptz,
  cooldown_until        timestamptz,
  failed_session_count  integer NOT NULL DEFAULT 0,
  last_reset_by         text,
  last_reset_at         timestamptz,
  is_open               boolean NOT NULL DEFAULT false
);

CREATE TABLE symptom_session (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  symptom_id      uuid NOT NULL,
  fingerprint     text NOT NULL,
  agent_session_id uuid,
  status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'spawned', 'terminal_succeeded', 'terminal_failed', 'hitl_queued')),
  trigger_reason  text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX symptom_session_fingerprint_idx ON symptom_session(fingerprint, created_at DESC);
CREATE INDEX symptom_session_agent_idx ON symptom_session(agent_session_id) WHERE agent_session_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS symptom_session;
DROP TABLE IF EXISTS circuit_breaker_state;
DROP TABLE IF EXISTS noise_rule;
