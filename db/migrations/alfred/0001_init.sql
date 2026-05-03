-- +goose Up
-- Alfred sessions, messages and decision log for Phase 1.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE alfred_session (
  id                 uuid PRIMARY KEY,
  workspace_id       uuid NOT NULL,
  actor              text NOT NULL,
  started_at         timestamptz NOT NULL,
  last_activity_at   timestamptz NOT NULL,
  status             text NOT NULL CHECK (status IN ('open', 'closed')),
  correlation_id     text NOT NULL,
  metadata           jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE alfred_message (
  id             uuid PRIMARY KEY,
  session_id     uuid NOT NULL REFERENCES alfred_session(id) ON DELETE CASCADE,
  role           text NOT NULL CHECK (role IN ('user', 'assistant', 'system', 'tool')),
  content        text NOT NULL,
  tool_call_id   text,
  created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE alfred_decision (
  id                 uuid PRIMARY KEY,
  session_id          uuid NOT NULL REFERENCES alfred_session(id) ON DELETE CASCADE,
  workspace_id        uuid NOT NULL,
  actor               text NOT NULL,
  correlation_id      text NOT NULL,
  intent              text NOT NULL,
  retrieved_refs      jsonb NOT NULL DEFAULT '[]'::jsonb,
  policy_evaluated    jsonb,
  tool_kind           text CHECK (tool_kind IN ('mcp', 'skill', 'prompt', 'llm', 'delegation')),
  tool_id             text,
  params_redacted     jsonb NOT NULL DEFAULT '{}'::jsonb,
  outcome             text NOT NULL CHECK (outcome IN ('pending', 'running', 'succeeded', 'failed', 'approved', 'rejected', 'expired')),
  outcome_detail      jsonb,
  occurred_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX alfred_session_workspace_idx ON alfred_session(workspace_id);
CREATE INDEX alfred_session_correlation_idx ON alfred_session(correlation_id);
CREATE INDEX alfred_message_session_idx ON alfred_message(session_id, created_at);
CREATE INDEX alfred_decision_workspace_idx ON alfred_decision(workspace_id, occurred_at DESC);
CREATE INDEX alfred_decision_session_idx ON alfred_decision(session_id, occurred_at DESC);
CREATE INDEX alfred_decision_correlation_idx ON alfred_decision(correlation_id);

-- +goose Down
DROP TABLE IF EXISTS alfred_decision;
DROP TABLE IF EXISTS alfred_message;
DROP TABLE IF EXISTS alfred_session;
