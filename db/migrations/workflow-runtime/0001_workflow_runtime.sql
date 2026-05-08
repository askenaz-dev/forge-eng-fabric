-- workflow-runtime schema (Phase 5)
--
-- Stores durable execution metadata and a per-step event log. Designed to be
-- mirrored from the Temporal cluster — Temporal remains source of truth for
-- replay; these tables are queryable views for the Portal and analytics.

CREATE TABLE IF NOT EXISTS workflow_execution (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL,
    workspace_id    TEXT NOT NULL,
    namespace       TEXT NOT NULL,
    workflow_id     TEXT NOT NULL,
    version         TEXT NOT NULL,
    correlation_id  TEXT,
    status          TEXT NOT NULL CHECK (status IN ('pending','running','waiting','completed','failed','cancelled','compensating')),
    inputs          JSONB,
    outputs         JSONB,
    failure_reason  TEXT,
    dry_run         BOOLEAN NOT NULL DEFAULT FALSE,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS workflow_execution_tenant_idx
    ON workflow_execution (tenant_id, workspace_id, started_at DESC);
CREATE INDEX IF NOT EXISTS workflow_execution_workflow_idx
    ON workflow_execution (workflow_id, version, started_at DESC);
CREATE INDEX IF NOT EXISTS workflow_execution_status_idx
    ON workflow_execution (status, updated_at DESC);

CREATE TABLE IF NOT EXISTS workflow_step_event (
    id              BIGSERIAL PRIMARY KEY,
    execution_id    TEXT NOT NULL REFERENCES workflow_execution(id) ON DELETE CASCADE,
    step_id         TEXT NOT NULL,
    step_type       TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending','running','waiting','completed','failed','compensated','skipped')),
    attempt         INTEGER NOT NULL DEFAULT 1,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    inputs          JSONB,
    outputs         JSONB,
    failure_reason  TEXT
);

CREATE INDEX IF NOT EXISTS workflow_step_event_exec_idx
    ON workflow_step_event (execution_id, step_id, attempt);
CREATE INDEX IF NOT EXISTS workflow_step_event_status_idx
    ON workflow_step_event (status, started_at DESC);

CREATE TABLE IF NOT EXISTS workflow_compensation (
    id              BIGSERIAL PRIMARY KEY,
    execution_id    TEXT NOT NULL REFERENCES workflow_execution(id) ON DELETE CASCADE,
    for_step_id     TEXT NOT NULL,
    compensate_id   TEXT NOT NULL,
    outcome         TEXT NOT NULL,
    at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS workflow_compensation_exec_idx
    ON workflow_compensation (execution_id, at DESC);
