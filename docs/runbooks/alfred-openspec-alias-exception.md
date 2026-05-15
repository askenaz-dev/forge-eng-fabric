# `/openspec` Alias Exception Runbook

The `/openspec` command is a deprecated alias for `/forge`. It is scheduled for removal after two minor versions. This runbook covers how to grant a tenant an extension if they cannot migrate in time.

## When to grant an exception

- The tenant has tooling or CI pipelines that hard-code `/openspec` commands and cannot be migrated within the standard two-minor-version window.
- A formal request has been submitted and reviewed by the Platform Architecture team.

## Granting the exception

Set `force_keep_openspec_alias: true` in the tenant config:

```sh
curl -X PATCH http://<control-plane>/v1/tenants/<tenant_id>/config \
  -H 'Authorization: Bearer <token>' \
  -H 'content-type: application/json' \
  -d '{"force_keep_openspec_alias": true}'
```

The Alfred command router reads this config flag on each request. When `true`, `/openspec` commands route to `/forge` without showing the deprecation toast or emitting `alfred.command.deprecated_alias.v1`.

**Record the exception**: add a row to `docs/governance/openspec-alias-exceptions.md` with the tenant ID, requester, justification, expiry date, and migration plan.

## Expiry and follow-up

Exceptions are granted for a maximum of one additional minor version (i.e., removal is deferred to v+3 instead of v+2 for that tenant).

At each release cut, the release manager must:

1. Check `docs/governance/openspec-alias-exceptions.md` for expired entries.
2. Notify the tenant at least 4 weeks before their expiry date.
3. If the tenant is unresponsive or still unprepared past the expiry, escalate to Platform Architecture before proceeding with removal.

## Revoking the exception

When the tenant has migrated:

```sh
curl -X PATCH http://<control-plane>/v1/tenants/<tenant_id>/config \
  -H 'Authorization: Bearer <token>' \
  -H 'content-type: application/json' \
  -d '{"force_keep_openspec_alias": false}'
```

Remove the entry from `docs/governance/openspec-alias-exceptions.md` and note the migration completion date.

## Monitoring exceptions in production

The Grafana "Deprecated alias volume" panel (`alfred.command.deprecated_alias.v1` event count) breaks down by tenant. Tenants with active exceptions should show zero events (since the event is suppressed). Non-zero counts from an excepted tenant indicate the exception flag may not be taking effect — investigate the Alfred config cache.
