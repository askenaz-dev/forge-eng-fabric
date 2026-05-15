# `/openspec` Alias Removal Schedule

The `/openspec` command is a deprecated alias for `/forge`, introduced in the alfred-console-redesign change. It is scheduled for removal after two minor versions from the release that introduced the `/forge` rename.

## Timeline

| Milestone | Target | Status |
|---|---|---|
| `/forge` canonical + `/openspec` alias introduced | v0.X.0 (current) | ✓ Shipped |
| Alias emits `alfred.command.deprecated_alias.v1` + yellow toast | v0.X.0 (current) | ✓ Shipped |
| CLI `openspec` command prints stderr deprecation warning | v0.X.0 (current) | ✓ Shipped |
| **Alias removal** | **v0.(X+2).0** | Scheduled |

## Removal checklist (for release v0.(X+2).0)

Before removing the `/openspec` alias:

- [ ] Check `docs/governance/openspec-alias-exceptions.md` — all exceptions must have expired or been resolved.
- [ ] Verify `alfred.command.deprecated_alias.v1` event volume has trended to zero for ≥ 4 weeks across all non-excepted tenants.
- [ ] Send deprecation reminder to all tenant admin contacts 4 weeks before the release cut.
- [ ] Remove `openspecRoot` command tree from `cli/forge/cmd/forge/main.go`.
- [ ] Remove `openspecDeprecated()` helper from the CLI.
- [ ] Remove the `/openspec new (deprecated)` entry from `CommandPalette.tsx` `forgeActions`.
- [ ] Remove the `forge-command.deprecated` case from `CommandPalette.tsx` `runAction`.
- [ ] Remove `alfred.command.deprecated_alias.v1` from the EVENT_TYPES registry in `services/alfred/alfred/agent_mode/events.py` if no other consumers exist.
- [ ] Update the changelog and release notes.

## Exception tracking

Active tenant exceptions are tracked in `docs/governance/openspec-alias-exceptions.md`.

| Tenant | Exception granted | Expiry | Migration plan |
|---|---|---|---|
| — | — | — | — |

## Monitoring

Track removal readiness in Grafana using the "Deprecated alias volume" panel. The goal is zero volume for 4+ consecutive weeks before removing the alias.
