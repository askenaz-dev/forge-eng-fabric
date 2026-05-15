# Runbook — App-first-class-entity per-workspace cutover

> Audience: SDLC platform on-call. Run this once per pilot workspace, in
> sequence (M0 → M6) per the design's Migration Plan section.

## Pre-requisites

- The application service is deployed and healthy (`/healthz` returning 200).
- Migrations 0008/0002/0003/0002 are applied (registry, openspec, app-onboarding,
  runtime-registry) and `application_backfill_sentinel` exists but is empty.
- The `_unassigned` App is materialised for the target workspace via the
  bootstrap hook OR the migration job. Verify with:
  `SELECT id FROM application WHERE workspace_id=$1 AND slug='_unassigned';`
- The workspace owner has been notified ≥ 5 business days in advance of the
  destructive run.

## M0 — Dark launch (already shipped)

Confirm `forge.app_entity.enabled=false` and `forge.app_scope.wizard_step.enabled=false`
for the workspace. No mutations.

## M1 — Per-workspace dry run

```bash
spec-app-migration dry-run \
  --workspace=<workspace-id> \
  --source=/tmp/specs-snapshot.json \
  --out=/tmp/migration-out
```

Verify `migration-dry-run-<ws>-<ts>.csv` lists every spec with one of the three
classifications. Mail the CSV to the workspace owner along with the deletion
list and the 5-business-day clock.

## M2 — Owner confirmation

When the workspace owner replies with their signed confirmation token:

```bash
spec-app-migration confirm \
  --workspace=<workspace-id> \
  --report=/tmp/migration-out/migration-dry-run-<ws>-<ts>.csv \
  --signature=<signed-token>
```

Pipe the JSON output into `psql` to insert into `application_audit` with
`action='migration_confirmation'`. The execute step refuses to run until this
row exists.

## M3 — Backfill

```bash
FORGE_MIGRATION_CONFIRMATION=<signed-token> \
spec-app-migration execute \
  --workspace=<workspace-id> \
  --report=/tmp/migration-out/migration-dry-run-<ws>-<ts>.csv
```

Verify `backfilled=N purged=K` matches the dry-run totals. Spot-check 5
random backfilled specs in `openspec_index` to confirm `app_id` is set and
matches a row in `application`.

## M4 — Hard delete orphans

The execute step above hard-deletes orphans and emits `spec.purged.v1`. Verify
the immutable audit retention bucket has one object per purged spec at
`s3://forge-audit/forge.spec.purged/<spec_id>.json`.

## M5 — Flip NOT NULL

After confirming zero NULL `app_id` rows for the workspace, set the sentinel:

```sql
INSERT INTO application_backfill_sentinel (workspace_id, set_by, notes)
VALUES ('<workspace-id>', 'oncall:<sub>', 'pilot cutover');
```

Run the NOT NULL flip migrations:
- `db/migrations/registry/0009_app_id_not_null.sql`
- `db/migrations/openspec/0003_app_id_not_null.sql`

Then flip the feature flags for the workspace:
- `forge.app_entity.enabled=true`
- `forge.app_scope.wizard_step.enabled=true`

## M6 — Repeat per workspace

When all pilot workspaces are migrated, flip the flag defaults globally and
remove the per-workspace gates from the feature flag service.

## Rollback

Up to M5: `flag off + drop app_id column`. After M5: re-introduce nullability
and restore purged orphans via `spec-app-migration restore-from-audit
--workspace=<ws> --spec-id=<id>`. Target ≤ 4h per workspace per Risk #1 in the
design.
