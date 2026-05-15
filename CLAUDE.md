# Forge Engineering Fabric — Claude Code Guidance

## Key commands

| Task | Command |
|---|---|
| Start local stack | `make up` |
| Run all tests | `make test` |
| Go service tests | `go test ./...` (from service dir) |
| Python service tests | `uv run --extra dev pytest -q` (from service dir) |
| Portal build | `cd portal && corepack pnpm build` |
| OpenSpec status | `openspec status --change "<name>" --json` |
| Apply OpenSpec change | `/opsx:apply <change-name>` |
| Demo intent-to-infra | `make demo-intent-to-infrastructure` |

## Alfred console commands

The canonical command is **`/forge`** (not `/openspec`). Use `/forge` in all code, docs, and examples.

| CLI | Portal palette |
|---|---|
| `forge new` | `/forge new` |
| `forge list` | `/forge list` |
| `forge edit <id>` | `/forge edit` |

`/openspec` is a deprecated alias that emits a warning and routes to `/forge`. Do not add new references to `/openspec` in code, tests, or documentation.

## Alfred service

- Location: `services/alfred/`
- Local port: `8090`
- Key env vars: `ALFRED_CONSOLE_V2_ENABLED`, `SPEC_MATCH_THRESHOLD_DEFAULT`, `SPEC_MATCH_THRESHOLD_FLOOR`, `DEDUP_INDEX_URL`
- OpenAPI contract: `contracts/openapi/alfred-agent-mode.yaml`

## Portal

- Location: `portal/`
- Framework: Next.js 14.2, React 18, TypeScript
- i18n dictionary: `portal/src/i18n/dictionary.ts` — all new copy goes here (ES default, EN fallback)
- Local port: `3000`

## OpenSpec workflow

Active changes live in `openspec/changes/`. Use `/opsx:apply <change>` to implement tasks and mark them complete. Archive with `/opsx:archive` when all tasks are done.

## Reference workflows

| Workflow | Command |
|---|---|
| `forge.reference.intent-to-infrastructure@1` | `make demo-intent-to-infrastructure` |
| `forge.reference.intent-to-deploy@1` | `make demo-intent-to-deploy` |

## Do not

- Reference `/openspec` in new code or documentation — use `/forge`.
- Commit secrets or `.env` files.
- Push directly to `main` without a PR.
