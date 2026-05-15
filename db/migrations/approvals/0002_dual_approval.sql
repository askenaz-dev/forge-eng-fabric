-- +goose Up
-- Extend approval_request with dual-approval semantics (iter 5).
-- approval_mode = 'any': any single required_approver suffices.
-- approval_mode = 'dual': ALL required_approvers must approve.
-- approvals_given tracks the set of principals who have approved so far.
-- self_revoke_window_secs: window in which an approver can retract their decision.

ALTER TABLE approval_request
  ADD COLUMN IF NOT EXISTS approval_mode          text    NOT NULL DEFAULT 'any'
    CHECK (approval_mode IN ('any', 'dual')),
  ADD COLUMN IF NOT EXISTS approvals_given        text[]  NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS self_revoke_window_secs int    NOT NULL DEFAULT 60,
  ADD COLUMN IF NOT EXISTS triggered_by_symptom_id uuid,
  ADD COLUMN IF NOT EXISTS triggered_by_session_id uuid;

CREATE INDEX approval_request_symptom_idx ON approval_request(triggered_by_symptom_id)
  WHERE triggered_by_symptom_id IS NOT NULL;

-- Helper view: returns requests in 'approved' status only when dual requirements met.
-- For 'any' mode: status 'approved' once decided_by IS NOT NULL.
-- For 'dual' mode: status 'approved' once every required_approver is in approvals_given.
CREATE OR REPLACE VIEW approval_request_effective AS
SELECT *,
  CASE
    WHEN approval_mode = 'any' THEN status
    WHEN approval_mode = 'dual' AND status = 'pending' AND
         (SELECT bool_and(a = ANY(approvals_given))
          FROM unnest(required_approvers) AS a) THEN 'approved'
    ELSE status
  END AS effective_status
FROM approval_request;

-- +goose Down
DROP VIEW IF EXISTS approval_request_effective;
ALTER TABLE approval_request
  DROP COLUMN IF EXISTS triggered_by_session_id,
  DROP COLUMN IF EXISTS triggered_by_symptom_id,
  DROP COLUMN IF EXISTS self_revoke_window_secs,
  DROP COLUMN IF EXISTS approvals_given,
  DROP COLUMN IF EXISTS approval_mode;
