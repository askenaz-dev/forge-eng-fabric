-- +goose Up
-- Platform-ops audit tables.
-- platform_ops_audit_event is the canonical audit log for all actions
-- taken by Alfred through the platform-ops service.
-- It deliberately mirrors the audit_event interface but lives in its own
-- table so it can carry autonomous-action-specific columns without
-- modifying the shared (partitioned) audit_event table.

CREATE TABLE platform_ops_audit_event (
  audit_id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         uuid,
  actor             text NOT NULL,
  actor_session     text,
  action            text NOT NULL,
  resource          text NOT NULL,
  outcome           text NOT NULL CHECK (outcome IN ('success','denied','error')),
  details           jsonb NOT NULL DEFAULT '{}',
  occurred_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX platform_ops_audit_event_actor_idx      ON platform_ops_audit_event(actor, occurred_at DESC);
CREATE INDEX platform_ops_audit_event_resource_idx   ON platform_ops_audit_event(resource, occurred_at DESC);
CREATE INDEX platform_ops_audit_event_occurred_idx   ON platform_ops_audit_event(occurred_at DESC);

-- platform_ops_audit_ext holds the autonomous-action supplements that
-- link a platform-ops action back to its originating symptom, the OPA
-- policy bundle in use, any post-validate verification result, and the
-- rollback action (if one was issued).

CREATE TABLE platform_ops_audit_ext (
  audit_id            uuid PRIMARY KEY REFERENCES platform_ops_audit_event(audit_id) ON DELETE CASCADE,
  symptom_id          uuid,
  agent_session_id    uuid,
  policy_bundle_hash  text,
  approvers           text[],
  verification        jsonb,
  rollback_action_id  uuid
);

CREATE INDEX platform_ops_audit_ext_symptom_idx     ON platform_ops_audit_ext(symptom_id)      WHERE symptom_id IS NOT NULL;
CREATE INDEX platform_ops_audit_ext_session_idx     ON platform_ops_audit_ext(agent_session_id) WHERE agent_session_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS platform_ops_audit_ext;
DROP TABLE IF EXISTS platform_ops_audit_event;
