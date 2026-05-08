-- Asset observability schema (Phase 5)
--
-- Supports per-asset metric queries. The hot path is in-memory; this table
-- mirrors invocations for analytics and long retention. ClickHouse can be
-- substituted at high volume — schema is intentionally compatible.

CREATE TABLE IF NOT EXISTS asset_invocation (
    id                  BIGSERIAL PRIMARY KEY,
    asset_id            TEXT NOT NULL,
    asset_type          TEXT NOT NULL CHECK (asset_type IN ('skill','prompt','workflow','mcp_tool')),
    asset_version       TEXT,
    tenant_id           TEXT NOT NULL,
    workspace_id        TEXT NOT NULL,
    started_at          TIMESTAMPTZ NOT NULL,
    duration_ms         DOUBLE PRECISION NOT NULL DEFAULT 0,
    success             BOOLEAN NOT NULL DEFAULT TRUE,
    llm_cost_usd        DOUBLE PRECISION NOT NULL DEFAULT 0,
    compute_cost_usd    DOUBLE PRECISION NOT NULL DEFAULT 0,
    eval_score          DOUBLE PRECISION,
    business_metric     DOUBLE PRECISION,
    business_metric_key TEXT,
    step_failures       TEXT[],
    correlation_id      TEXT
);

CREATE INDEX IF NOT EXISTS asset_invocation_asset_idx
    ON asset_invocation (asset_id, started_at DESC);
CREATE INDEX IF NOT EXISTS asset_invocation_tenant_idx
    ON asset_invocation (tenant_id, asset_type, started_at DESC);
