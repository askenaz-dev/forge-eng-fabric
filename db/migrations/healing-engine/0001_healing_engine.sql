-- Healing engine schema (Phase 6).
--
-- Tracks envelopes (autonomy boundaries), action invocations, kill-switch
-- state, and promotion-eligibility metrics. The hot path is in-memory; this
-- mirror is for analytics, audit and replay.

CREATE TABLE IF NOT EXISTS healing_envelope (
    id                  TEXT PRIMARY KEY,
    tenant_id           TEXT NOT NULL,
    workspace_id        TEXT,
    capability          TEXT NOT NULL,
    asset_pattern       TEXT,
    environment         TEXT NOT NULL,
    criticality         TEXT NOT NULL,
    default_level       TEXT NOT NULL CHECK (default_level IN ('L1','L2','L3','L4','L5')),
    allowed_levels      TEXT[] NOT NULL,
    time_windows        TEXT[],
    max_actions_per_hour INTEGER NOT NULL DEFAULT 0,
    kill_switch         BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS healing_envelope_lookup_idx
    ON healing_envelope (capability, environment, criticality);

CREATE TABLE IF NOT EXISTS healing_action_invocation (
    id              TEXT PRIMARY KEY,
    incident_id     TEXT NOT NULL,
    action_id       TEXT NOT NULL,
    envelope_id     TEXT,
    requested_level TEXT NOT NULL,
    applied_level   TEXT NOT NULL,
    outcome         TEXT NOT NULL,
    reason          TEXT,
    workflow_run_id TEXT,
    approval_id     TEXT,
    synthetic       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS healing_invocation_incident_idx
    ON healing_action_invocation (incident_id, created_at DESC);

CREATE TABLE IF NOT EXISTS healing_kill_switch (
    workspace_id    TEXT PRIMARY KEY,
    active          BOOLEAN NOT NULL,
    actor           TEXT,
    reason          TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS healing_action_promotion_stats (
    action_id                   TEXT NOT NULL,
    environment                 TEXT NOT NULL,
    eval_pass_rate_last_50      DOUBLE PRECISION NOT NULL DEFAULT 0,
    successful_l3_runs          INTEGER NOT NULL DEFAULT 0,
    days_since_last_postmortem  INTEGER NOT NULL DEFAULT 0,
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (action_id, environment)
);
