-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE sdlc_initiative (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id   text NOT NULL,
  openspec_root  text NOT NULL,
  jira_epic_key  text,
  criticality    text NOT NULL DEFAULT 'medium',
  current_phase  text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sdlc_phase_state (
  initiative_id uuid NOT NULL REFERENCES sdlc_initiative(id) ON DELETE CASCADE,
  phase         text NOT NULL,
  status        text NOT NULL,
  entered_at    timestamptz,
  completed_at  timestamptz,
  PRIMARY KEY (initiative_id, phase)
);

CREATE TABLE sdlc_gate_result (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  initiative_id uuid NOT NULL REFERENCES sdlc_initiative(id) ON DELETE CASCADE,
  phase         text NOT NULL,
  gate          text NOT NULL,
  outcome       text NOT NULL,
  reason        text,
  detail        jsonb NOT NULL DEFAULT '{}'::jsonb,
  evaluated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sdlc_blocker (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  initiative_id uuid NOT NULL REFERENCES sdlc_initiative(id) ON DELETE CASCADE,
  phase         text NOT NULL,
  gate          text NOT NULL,
  reason        text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  resolved_at   timestamptz
);

CREATE INDEX sdlc_initiative_workspace_idx ON sdlc_initiative(workspace_id);
CREATE INDEX sdlc_initiative_openspec_idx ON sdlc_initiative(openspec_root);
CREATE INDEX sdlc_gate_result_initiative_phase_idx ON sdlc_gate_result(initiative_id, phase);
CREATE INDEX sdlc_blocker_open_idx ON sdlc_blocker(initiative_id, phase) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS sdlc_blocker;
DROP TABLE IF EXISTS sdlc_gate_result;
DROP TABLE IF EXISTS sdlc_phase_state;
DROP TABLE IF EXISTS sdlc_initiative;
