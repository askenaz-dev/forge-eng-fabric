-- Phase 5: workflow asset sub-resources (versions, eval_run, installation)
--
-- The workflow assets themselves continue to live in the asset table; the
-- sub-resources are stored separately and joined at read time. This avoids
-- bloating the asset row JSON columns and keeps each sub-resource queryable.

ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_type_check;
ALTER TABLE asset ADD CONSTRAINT asset_type_check
    CHECK (type IN ('mcp','skill','agent','workflow','prompt_template','application','repo_template','eval_dataset'));

CREATE TABLE IF NOT EXISTS asset_workflow_version (
    asset_id        TEXT NOT NULL,
    version         TEXT NOT NULL,
    workflow_id     TEXT NOT NULL,
    workflow_version TEXT NOT NULL,
    ast             JSONB NOT NULL,
    diff_prev       JSONB,
    lifecycle_state TEXT NOT NULL DEFAULT 'draft',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, version, workflow_version)
);

CREATE INDEX IF NOT EXISTS asset_workflow_version_workflow_idx
    ON asset_workflow_version (workflow_id, workflow_version);

CREATE TABLE IF NOT EXISTS asset_workflow_eval_run (
    id              TEXT PRIMARY KEY,
    asset_id        TEXT NOT NULL,
    workflow_version TEXT NOT NULL,
    eval_run_id     TEXT NOT NULL,
    outcome         TEXT NOT NULL,
    metric_value    DOUBLE PRECISION NOT NULL DEFAULT 0,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS asset_workflow_eval_run_asset_idx
    ON asset_workflow_eval_run (asset_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS asset_workflow_installation (
    id                  TEXT PRIMARY KEY,
    asset_id            TEXT NOT NULL,
    workflow_version    TEXT NOT NULL,
    target_workspace_id TEXT NOT NULL,
    source_workspace_id TEXT NOT NULL,
    listing_id          TEXT,
    status              TEXT NOT NULL DEFAULT 'active',
    installed_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS asset_workflow_installation_asset_idx
    ON asset_workflow_installation (asset_id, target_workspace_id);
