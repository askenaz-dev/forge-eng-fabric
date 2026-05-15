-- +goose Up
-- Phase 5: add nullable app_id to app_onboarding_request and rotate the
-- idempotency key to (workspace_id, app_id, repo_name). The old
-- (workspace_id, repo_name) constraint is dropped because two Apps inside the
-- same workspace are now allowed to onboard the same upstream repo (e.g., a
-- monorepo backing both a customer-portal App and an internal-tools App).

ALTER TABLE app_onboarding_request
  ADD COLUMN IF NOT EXISTS app_id uuid;

CREATE INDEX IF NOT EXISTS app_onboarding_request_app_idx
  ON app_onboarding_request (app_id);

ALTER TABLE app_onboarding_request
  DROP CONSTRAINT IF EXISTS app_onboarding_request_workspace_id_repo_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS app_onboarding_request_ws_app_repo_uidx
  ON app_onboarding_request (workspace_id, app_id, repo_name)
  WHERE app_id IS NOT NULL;

-- Until app_id is backfilled for every legacy row, keep the legacy uniqueness
-- as a partial index so existing callers stay coherent.
CREATE UNIQUE INDEX IF NOT EXISTS app_onboarding_request_ws_repo_legacy_uidx
  ON app_onboarding_request (workspace_id, repo_name)
  WHERE app_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS app_onboarding_request_ws_repo_legacy_uidx;
DROP INDEX IF EXISTS app_onboarding_request_ws_app_repo_uidx;
ALTER TABLE app_onboarding_request
  ADD CONSTRAINT app_onboarding_request_workspace_id_repo_name_key
  UNIQUE (workspace_id, repo_name);
DROP INDEX IF EXISTS app_onboarding_request_app_idx;
ALTER TABLE app_onboarding_request DROP COLUMN IF EXISTS app_id;
