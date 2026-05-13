-- +goose Up
-- Alfred agent-mode sessions: long-running, plan-driven orchestrator state.
-- See openspec/changes/alfred-agent-mode-orchestrator/design.md (D2, D4).

CREATE TABLE alfred_agent_session (
  id                       uuid PRIMARY KEY,
  workspace_id             uuid NOT NULL,
  openspec_id              text,
  correlation_id           text NOT NULL,
  originator_principal     text NOT NULL,
  model_id                 text NOT NULL,
  plan_revision            integer NOT NULL DEFAULT 1,
  plan_json                jsonb NOT NULL DEFAULT '{}'::jsonb,
  frozen_autonomy_policy   jsonb NOT NULL DEFAULT '{}'::jsonb,
  status                   text NOT NULL CHECK (status IN (
                              'planning', 'running', 'paused_for_approval',
                              'paused_for_budget', 'completed', 'aborted', 'failed'
                           )),
  started_at               timestamptz NOT NULL DEFAULT now(),
  paused_at                timestamptz,
  resumed_at               timestamptz,
  completed_at             timestamptz,
  aborted_reason           text,
  workflow_run_id          text
);

CREATE TABLE alfred_agent_step (
  id              uuid PRIMARY KEY,
  session_id      uuid NOT NULL REFERENCES alfred_agent_session(id) ON DELETE CASCADE,
  idx             integer NOT NULL,
  kind            text NOT NULL CHECK (kind IN ('plan', 'tool', 'workflow', 'agent', 'approval', 'final')),
  tool_id         text,
  workflow_id     text,
  agent_id        text,
  criticality     text NOT NULL DEFAULT 'low' CHECK (criticality IN ('low', 'medium', 'high', 'critical')),
  decision_id     uuid REFERENCES alfred_decision(id) ON DELETE SET NULL,
  status          text NOT NULL CHECK (status IN (
                     'pending', 'running', 'paused_for_approval', 'paused_for_budget',
                     'succeeded', 'failed', 'skipped', 'cancelled'
                  )),
  started_at      timestamptz,
  completed_at    timestamptz,
  outcome         jsonb
);

CREATE INDEX alfred_agent_session_workspace_status_idx
  ON alfred_agent_session(workspace_id, status, started_at DESC);
CREATE INDEX alfred_agent_session_originator_idx
  ON alfred_agent_session(originator_principal);
CREATE INDEX alfred_agent_session_correlation_idx
  ON alfred_agent_session(correlation_id);
CREATE INDEX alfred_agent_step_session_idx
  ON alfred_agent_step(session_id, idx);

-- +goose Down
DROP TABLE IF EXISTS alfred_agent_step;
DROP TABLE IF EXISTS alfred_agent_session;
