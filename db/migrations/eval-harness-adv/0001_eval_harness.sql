-- Advanced eval harness schema (Phase 5)

CREATE TABLE IF NOT EXISTS eval_dataset (
    asset_id        TEXT NOT NULL,
    version         TEXT NOT NULL,
    tenant_id       TEXT NOT NULL,
    workspace_id    TEXT NOT NULL,
    description     TEXT,
    trust_level     TEXT NOT NULL DEFAULT 'internal',
    items           JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, version)
);

CREATE TABLE IF NOT EXISTS workflow_eval_run (
    id                       TEXT PRIMARY KEY,
    tenant_id                TEXT NOT NULL,
    workspace_id             TEXT NOT NULL,
    workflow_id              TEXT NOT NULL,
    workflow_version         TEXT NOT NULL,
    dataset_id               TEXT NOT NULL,
    dataset_version          TEXT NOT NULL,
    outcome                  TEXT NOT NULL DEFAULT 'passed'
        CHECK (outcome IN ('passed','failed','regression_blocked')),
    metric_key               TEXT NOT NULL DEFAULT 'success_rate',
    metric_value             DOUBLE PRECISION NOT NULL DEFAULT 0,
    baseline_value           DOUBLE PRECISION,
    delta_threshold          DOUBLE PRECISION NOT NULL DEFAULT 0.03,
    items                    INTEGER NOT NULL DEFAULT 0,
    failures                 INTEGER NOT NULL DEFAULT 0,
    cost_usd                 DOUBLE PRECISION,
    latency_p95_ms           DOUBLE PRECISION,
    business_metric_value    DOUBLE PRECISION,
    started_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at             TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS workflow_eval_run_workflow_idx
    ON workflow_eval_run (workflow_id, workflow_version, started_at DESC);
CREATE INDEX IF NOT EXISTS workflow_eval_run_outcome_idx
    ON workflow_eval_run (outcome, started_at DESC);

CREATE TABLE IF NOT EXISTS workflow_ab_run (
    id                  TEXT PRIMARY KEY,
    tenant_id           TEXT NOT NULL,
    workspace_id        TEXT NOT NULL,
    workflow_id         TEXT NOT NULL,
    version_a           TEXT NOT NULL,
    version_b           TEXT NOT NULL,
    target_executions   INTEGER NOT NULL DEFAULT 200,
    counts              JSONB NOT NULL DEFAULT '{"a":0,"b":0}'::jsonb,
    metrics             JSONB NOT NULL DEFAULT '{}'::jsonb,
    significant         BOOLEAN NOT NULL DEFAULT FALSE,
    completed           BOOLEAN NOT NULL DEFAULT FALSE,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ
);
