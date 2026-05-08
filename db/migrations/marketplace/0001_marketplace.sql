-- Marketplace schema (Phase 5)

CREATE TABLE IF NOT EXISTS marketplace_listing (
    id                 TEXT PRIMARY KEY,
    tenant_id          TEXT NOT NULL,
    workspace_id       TEXT NOT NULL,
    workflow_id        TEXT NOT NULL,
    version            TEXT NOT NULL,
    name               TEXT NOT NULL,
    description        TEXT,
    tags               TEXT[],
    criticality        TEXT,
    visibility         TEXT NOT NULL CHECK (visibility IN ('private','workspace','tenant','forge-certified')),
    approval_state     TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_state IN ('not_required','pending','approved','rejected')),
    eval_run_id        TEXT,
    eval_outcome       TEXT,
    security_review_id TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, workflow_id, version, workspace_id)
);

CREATE INDEX IF NOT EXISTS marketplace_listing_tenant_idx
    ON marketplace_listing (tenant_id, visibility, approval_state, updated_at DESC);
CREATE INDEX IF NOT EXISTS marketplace_listing_workflow_idx
    ON marketplace_listing (workflow_id, version);

CREATE TABLE IF NOT EXISTS workflow_install (
    id                  TEXT PRIMARY KEY,
    tenant_id           TEXT NOT NULL,
    target_workspace_id TEXT NOT NULL,
    source_workspace_id TEXT NOT NULL,
    workflow_id         TEXT NOT NULL,
    version             TEXT NOT NULL,
    listing_id          TEXT NOT NULL REFERENCES marketplace_listing(id) ON DELETE RESTRICT,
    status              TEXT NOT NULL DEFAULT 'active',
    installed_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    installed_by        TEXT
);

CREATE INDEX IF NOT EXISTS workflow_install_workspace_idx
    ON workflow_install (tenant_id, target_workspace_id, installed_at DESC);
CREATE INDEX IF NOT EXISTS workflow_install_workflow_idx
    ON workflow_install (workflow_id, version);
