-- Incident detection schema (Phase 6)
--
-- The detection layer normalises external alerts (Prometheus / Cloud Monitoring
-- / Loki) and internal CloudEvents into a canonical incident stream consumed by
-- the diagnosis pipeline and healing engine. Hot path is in-memory; this schema
-- is the durable mirror for analytics, dedup history and replay.

CREATE TABLE IF NOT EXISTS incident (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL,
    workspace_id    TEXT,
    service         TEXT NOT NULL,
    environment     TEXT NOT NULL,
    signature_hash  TEXT NOT NULL,
    source          TEXT NOT NULL CHECK (source IN ('prometheus','cloud-monitoring','loki','manual','internal-event')),
    severity        TEXT NOT NULL CHECK (severity IN ('info','warning','critical')),
    title           TEXT NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL CHECK (status IN ('open','resolved')),
    opened_at       TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL,
    resolved_at     TIMESTAMPTZ,
    labels          JSONB,
    synthetic       BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS incident_open_idx
    ON incident (status, service, environment, signature_hash)
    WHERE status = 'open';

CREATE INDEX IF NOT EXISTS incident_tenant_idx
    ON incident (tenant_id, opened_at DESC);

CREATE TABLE IF NOT EXISTS incident_event (
    id              TEXT PRIMARY KEY,
    incident_id     TEXT NOT NULL REFERENCES incident(id) ON DELETE CASCADE,
    source          TEXT NOT NULL,
    severity        TEXT NOT NULL,
    occurred_at     TIMESTAMPTZ NOT NULL,
    payload         JSONB,
    labels          JSONB
);

CREATE INDEX IF NOT EXISTS incident_event_idx
    ON incident_event (incident_id, occurred_at DESC);
