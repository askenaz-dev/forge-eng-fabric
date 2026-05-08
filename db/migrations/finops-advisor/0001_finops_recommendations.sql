-- FinOps advisor schema (Phase 6).
--
-- Persists cost-reduction recommendations and the PRs they produce. The
-- advisor itself runs daily and refreshes this table via inserts; the Portal
-- module reads it for the FinOps Recommendations view.

CREATE TABLE IF NOT EXISTS finops_recommendation (
    id                              TEXT PRIMARY KEY,
    tenant_id                       TEXT NOT NULL,
    workspace_id                    TEXT,
    asset_id                        TEXT,
    kind                            TEXT NOT NULL CHECK (kind IN (
        'downsize_resource','idle_resource','oversized_resource',
        'expensive_llm_skill','cacheable_prompt'
    )),
    title                           TEXT NOT NULL,
    detail                          TEXT NOT NULL,
    expected_savings_usd_monthly    DOUBLE PRECISION NOT NULL DEFAULT 0,
    affected_resources              JSONB NOT NULL DEFAULT '[]',
    pr_url                          TEXT,
    pr_status                       TEXT NOT NULL DEFAULT 'draft',
    severity                        TEXT NOT NULL DEFAULT 'medium',
    metadata                        JSONB NOT NULL DEFAULT '{}',
    synthetic                       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS finops_recommendation_tenant_idx
    ON finops_recommendation (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS finops_recommendation_asset_idx
    ON finops_recommendation (asset_id, created_at DESC);
