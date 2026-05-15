# Runbook — Orphan-restore from audit retention

> Audience: SDLC platform on-call. Use within 30 days of a wrongful deletion
> (Risk #1 in app-first-class-entity/design.md). After 30 days the audit
> retention bucket may have rotated; escalate to platform-storage.

## When to use

A workspace owner reports a spec that was hard-deleted during the M3-M4
migration and should not have been (e.g. it had been pinned to a dashboard
the migration's pinned-set query missed). The audit retention bucket holds
the full prior body for 7 years.

## Inputs

- `spec_id`  — the openspec_id of the wrongly purged spec.
- `workspace_id` — the parent workspace.
- Approval ticket (one workspace-owner sign-off plus one platform-admin
  sign-off). Without both, do not proceed.

## Steps

### 1. Fetch the audit copy

```bash
aws s3 cp s3://forge-audit/forge.spec.purged/<spec_id>.json /tmp/restore-<spec_id>.json
jq '.workspace_id, .app_id, .audit.created_at' /tmp/restore-<spec_id>.json
```

Confirm the workspace and the original deletion timestamp match the
approval ticket.

### 2. Re-insert the row

```sql
BEGIN;
INSERT INTO openspec_index (openspec_id, workspace_id, app_id, title, business_intent,
                            version, path, updated_at)
SELECT openspec_id, workspace_id::uuid, app_id::uuid, title, business_intent,
       version, path, now()
FROM jsonb_populate_record(NULL::openspec_index,
  pg_read_binary_file('/tmp/restore-<spec_id>.json')::jsonb);
-- Verify exactly one row written.
COMMIT;
```

### 3. Record the restore in the audit trail

```sql
INSERT INTO application_audit (app_id, workspace_id, tenant_id, action, actor,
                               correlation_id, reason, after)
VALUES ('<app_id>', '<workspace_id>', '<tenant_id>', 'reparent_in',
        'oncall:<sub>', '<incident-id>', 'restore_from_audit',
        '<full-spec-body>'::jsonb);
```

### 4. Notify the workspace owner

Post a confirmation in the workspace owner's inbox channel with the new
record's `openspec_id` and the URL to the App detail page.

### 5. Postmortem

Open a postmortem ticket against the migration's orphan classifier. The
restore should never happen — if it does, the classifier missed an
evidence type and we update the rule before the next pilot.
