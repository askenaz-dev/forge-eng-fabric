-- workflow-registry schema (Phase 5)
--
-- The workflow_version row is **immutable**: a trigger refuses UPDATE on
-- rows whose lifecycle_state is 'published'.

CREATE TABLE IF NOT EXISTS workflow (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL,
    workspace_id    TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    owners          TEXT[],
    tags            TEXT[],
    visibility      TEXT NOT NULL DEFAULT 'private'
        CHECK (visibility IN ('private','workspace','tenant','forge-certified')),
    latest_version  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS workflow_tenant_idx
    ON workflow (tenant_id, workspace_id, created_at DESC);

CREATE TABLE IF NOT EXISTS workflow_version (
    workflow_id     TEXT NOT NULL REFERENCES workflow(id) ON DELETE CASCADE,
    version         TEXT NOT NULL,
    ast             JSONB NOT NULL,
    lifecycle_state TEXT NOT NULL DEFAULT 'draft'
        CHECK (lifecycle_state IN ('draft','in_review','approved','published','deprecated')),
    diff_prev       JSONB,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      TEXT,
    PRIMARY KEY (workflow_id, version)
);

CREATE INDEX IF NOT EXISTS workflow_version_state_idx
    ON workflow_version (lifecycle_state, published_at DESC);

-- Immutability: published versions cannot be mutated, including their
-- AST or diff. Allow lifecycle transition from published to deprecated only.
CREATE OR REPLACE FUNCTION workflow_version_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.lifecycle_state = 'published' THEN
        IF NEW.ast IS DISTINCT FROM OLD.ast OR
           NEW.diff_prev IS DISTINCT FROM OLD.diff_prev OR
           NEW.workflow_id IS DISTINCT FROM OLD.workflow_id OR
           NEW.version IS DISTINCT FROM OLD.version THEN
            RAISE EXCEPTION 'workflow_version_immutable_published';
        END IF;
        IF NEW.lifecycle_state NOT IN ('published', 'deprecated') THEN
            RAISE EXCEPTION 'workflow_version_lifecycle_invalid_transition';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS workflow_version_immutability ON workflow_version;
CREATE TRIGGER workflow_version_immutability
    BEFORE UPDATE ON workflow_version
    FOR EACH ROW EXECUTE FUNCTION workflow_version_immutable();
