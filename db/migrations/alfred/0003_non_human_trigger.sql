-- +goose Up
-- Non-human trigger fields for Alfred agent-mode sessions.
-- Added for the autonomous-platform-ops change (D3, D12).

ALTER TABLE alfred_agent_session
  ADD COLUMN IF NOT EXISTS trigger_source text NOT NULL DEFAULT 'human'
    CHECK (trigger_source IN ('human', 'symptom', 'playbook', 'replan')),
  ADD COLUMN IF NOT EXISTS actor text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS actor_session text,
  ADD COLUMN IF NOT EXISTS symptom_id uuid,
  ADD COLUMN IF NOT EXISTS playbook_id text,
  ADD COLUMN IF NOT EXISTS parent_session_id uuid REFERENCES alfred_agent_session(id);

CREATE INDEX alfred_agent_session_symptom_idx
  ON alfred_agent_session(symptom_id) WHERE symptom_id IS NOT NULL;
CREATE INDEX alfred_agent_session_actor_idx
  ON alfred_agent_session(actor, started_at DESC) WHERE actor != '';
CREATE INDEX alfred_agent_session_trigger_idx
  ON alfred_agent_session(trigger_source, started_at DESC) WHERE trigger_source != 'human';

-- +goose Down
ALTER TABLE alfred_agent_session
  DROP COLUMN IF EXISTS parent_session_id,
  DROP COLUMN IF EXISTS playbook_id,
  DROP COLUMN IF EXISTS symptom_id,
  DROP COLUMN IF EXISTS actor_session,
  DROP COLUMN IF EXISTS actor,
  DROP COLUMN IF EXISTS trigger_source;
