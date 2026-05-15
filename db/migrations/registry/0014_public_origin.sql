-- +goose Up
-- public-origin mirror flow: track assets that originated from a public
-- registry (npm, GitHub public) and were mirrored into tenant private storage.
-- Also adds auto_promote_policy for controlling automatic promotion of
-- mirrored assets that pass policy gates.

-- Task 13.2: origin tracking on the asset (version) row.
-- origin_ref:       canonical reference to the upstream source, e.g. "npm:my-skill@1.2.3"
-- is_public_origin: true when the asset bytes came from a public registry
-- last_synced_at:   timestamp of the last successful mirror/sync from the origin
ALTER TABLE asset
  ADD COLUMN IF NOT EXISTS origin_ref       text,
  ADD COLUMN IF NOT EXISTS is_public_origin boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS last_synced_at   timestamptz;

-- Backfill: existing rows get is_public_origin=false (already the default).

-- Task 13.2 + 13.4: add 'mirrored' as a valid lifecycle_state.
-- Assets start in 'mirrored' when the mirror flow fetches and stores the bytes.
-- From 'mirrored' they may be approved (emits asset.version.promoted.v1) or rejected.
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_lifecycle_state_check;
ALTER TABLE asset ADD CONSTRAINT asset_lifecycle_state_check
  CHECK (lifecycle_state IN ('proposed','in_review','approved','deprecated','retired','mirrored','rejected'));

-- Task 13.5: auto_promote_policy controls automatic promotion of mirrored assets.
-- 'none'  — no automatic promotion; manual approval required (default).
-- 'patch' — auto-promote patch-level (x.y.Z) version bumps from mirrored sources.
-- 'minor' — auto-promote patch and minor (x.Y.z) version bumps from mirrored sources.
ALTER TABLE asset
  ADD COLUMN IF NOT EXISTS auto_promote_policy text NOT NULL DEFAULT 'none'
    CHECK (auto_promote_policy IN ('none', 'patch', 'minor'));

-- Index for the mirror cron and portal mirror-queue queries.
CREATE INDEX IF NOT EXISTS asset_public_origin_idx
  ON asset (tenant_id, is_public_origin, lifecycle_state)
  WHERE is_public_origin = true;

-- +goose Down
DROP INDEX IF EXISTS asset_public_origin_idx;

ALTER TABLE asset DROP COLUMN IF EXISTS auto_promote_policy;
ALTER TABLE asset DROP COLUMN IF EXISTS last_synced_at;
ALTER TABLE asset DROP COLUMN IF EXISTS is_public_origin;
ALTER TABLE asset DROP COLUMN IF EXISTS origin_ref;

-- Restore the pre-0014 lifecycle_state constraint (from 0002_phase1_lifecycle.sql).
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_lifecycle_state_check;
ALTER TABLE asset ADD CONSTRAINT asset_lifecycle_state_check
  CHECK (lifecycle_state IN ('proposed','in_review','approved','deprecated','retired'));
