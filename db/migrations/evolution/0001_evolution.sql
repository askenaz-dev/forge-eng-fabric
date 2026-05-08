-- Evolution loop schema (Phase 6).
--
-- Persists OpenSpec change proposals derived by the autonomous-loop. Once
-- accepted, the platform OpenSpec service tracks the converted change in its
-- own schema; this table retains the link for audit + metrics.

CREATE TABLE IF NOT EXISTS evolution_proposal (
    id                  TEXT PRIMARY KEY,
    incident_id         TEXT NOT NULL,
    tenant_id           TEXT NOT NULL,
    workspace_id        TEXT,
    asset_id            TEXT,
    postmortem_url      TEXT,
    source              TEXT NOT NULL DEFAULT 'autonomous-loop',
    skill_version       TEXT NOT NULL,
    status              TEXT NOT NULL CHECK (status IN ('draft','inbox','accepted','rejected','converted')),
    title               TEXT NOT NULL,
    why                 TEXT,
    suggestions         JSONB NOT NULL DEFAULT '[]',
    openspec_change_id  TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    synthetic           BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS evolution_proposal_status_idx
    ON evolution_proposal (tenant_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS evolution_proposal_incident_idx
    ON evolution_proposal (incident_id);
