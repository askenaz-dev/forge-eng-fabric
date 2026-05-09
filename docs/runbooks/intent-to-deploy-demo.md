# Intent-to-Deploy Demo Runbook

> Workflow: `forge.reference.intent-to-deploy@1.0.0`
> Last validated: 2026-05-09 (synthetic-only locally; full live run pending Phase 3 sign-off)
> Make target: `make demo-intent-to-deploy`

This runbook drives a complete intent-to-deploy traversal:

```
alfred → sdlc-orchestrator → scaffolder → app-onboarding → ci → deploy-orchestrator
```

It is the canonical demo for showing the platform's "From Intent to Infrastructure" promise.

## Prerequisites

| Component | Source | Why |
|---|---|---|
| Local compose stack up | `make up` | Postgres, Kafka, OpenFGA, LiteLLM |
| `services/openspec` running | `cd services/openspec && uv run uvicorn openspec_service.app:app --port 8083` | Wizard backing store |
| `services/alfred` running | `cd services/alfred && uv run uvicorn alfred.app:app --port 8090` | Dialogue API + LLM gateway |
| `ALFRED_DIALOGUE_API=enabled` | env on Alfred | Surfaces `/v1/intent/*` routes |
| `services/workflow-registry` running | `go run ./cmd` | Hosts `forge.reference.intent-to-deploy@1` |
| `services/workflow-runtime` running | `go run ./cmd` | Executes the workflow |
| `services/approvals` running | service binary | HITL gate API |
| At least one runtime registered | `runtime-registry` API | Target for the deploy step |

If any component is unreachable, the demo falls back to **synthetic mode** — it still emits the milestone event chain so the smoke test can validate the workflow shape.

## Environment setup

```sh
# 1. Start the compose stack
make up

# 2. Enable wizard mode on Alfred
export ALFRED_DIALOGUE_API=enabled

# 3. Confirm the seed workflow is registered
curl -s http://localhost:8094/v1/workflows | jq '.workflows[] | select(.id=="forge.reference.intent-to-deploy")'

# 4. Pick a workspace and runtime
export WORKSPACE_ID=$(curl -s http://localhost:8081/v1/workspaces | jq -r '.[0].id')
export RUNTIME_ID=$(curl -s http://localhost:8110/v1/runtimes | jq -r '.runtimes[0].id')
```

## Run the demo

```sh
make demo-intent-to-deploy
```

The Make target invokes [`scripts/demo_intent_to_deploy.py`](../../scripts/demo_intent_to_deploy.py) which:

1. Submits a canned intent through `POST /v1/intent/start`.
2. Drives the wizard non-interactively: stakeholders, requirements, constraints.
3. Commits the draft via `POST /v1/intent/{id}/commit`.
4. Triggers `forge.reference.intent-to-deploy@1` via `POST /v1/executions` on `workflow-runtime`.
5. Auto-approves the HITL gate using the documented test-only fixture (`X-Forge-Demo-Auto-Approve: true` header).
6. Writes a JSON report to `build/demo-intent-to-deploy/<timestamp>.json`.

### Expected step output

| Step | Expected output |
|---|---|
| `intent.start` | `draft.draft_id` returned, `status=drafting` |
| `intent.answer` × 2 | Each turn updates `completeness` toward `complete` |
| `intent.commit` | `openspec_id` returned, `lifecycle_status=committed` |
| `workflow.trigger` | `execution_id` returned plus a `milestones[]` list |
| `approval.auto_grant` | HTTP 200 from approvals; decision recorded |
| Final | `DEPLOY URL: <url>` printed; report at `build/...json` |

### Auto-approval fixture

The HITL gate at `prod-approval-gate` is configured to accept a demo-only header:

```
X-Forge-Demo-Auto-Approve: true
```

This header is honoured **only when `FORGE_DEMO_AUTO_APPROVE=enabled`** on the approvals service, and **never** in production environments. See [`services/approvals/internal/policy.go`](../../services/approvals/internal/policy.go) for the gate.

## Common failure modes

| Symptom | Diagnosis | Remediation |
|---|---|---|
| `intent.start` fails with 401 | Bearer token missing | Obtain a token from Keycloak: `curl -X POST $KEYCLOAK_TOKEN_URL ...` |
| `intent.start` fails with 403 | Caller lacks `workspace.member` | Add the OpenFGA tuple `user:<sub>#member@workspace:<ws>` |
| `intent.commit` fails with 400 "draft not commit-ready" | Required fields still empty | Inspect the report, look for sections where `status != complete` |
| `workflow.trigger` returns 404 | Workflow not seeded | Restart workflow-registry; it seeds from `services/workflow-registry/seeds/` |
| `approval.auto_grant` fails with 403 | Demo header not recognised | Set `FORGE_DEMO_AUTO_APPROVE=enabled` on the approvals service |
| `deploy` step fails | Target runtime unreachable | Run `make verify-runtime RUNTIME=$RUNTIME_ID` first |
| Demo prints "synthetic mode" | One or more services unreachable | Start the missing services (see Prerequisites). Synthetic mode is OK for smoke tests but does not deploy anything real. |

## Rollback

The reference flow has a built-in `on_failure` handler that posts an incident note via `registry:skill/sdlc-devops/post-incident-note@1.0.0`. To roll back a deploy that succeeded:

```sh
# 1. Identify the deploy
cat build/demo-intent-to-deploy/<timestamp>.json | jq '.deploy_url, .image_digest'

# 2. Delete the deployment via deploy-orchestrator
curl -X DELETE http://localhost:8112/v1/deployments/<deploy_id>

# 3. Optionally revoke the OpenSpec
curl -X PATCH http://localhost:8083/v1/openspecs/<openspec_id> \
  -d '{"updated_by": "operator", "autonomy_policy": {"default_mode": "restricted"}}'
```

## Validation date

This runbook was last validated on **2026-05-09** in synthetic-only mode. Live-mode validation against staging GKE is a Phase 3 sign-off task; see [`platform-gaps-closure` task 8.2](../../openspec/changes/platform-gaps-closure/tasks.md).

## Related

- [Workflow seed source](../../services/workflow-registry/seeds/forge.reference.intent-to-deploy.v1.yaml)
- [Demo script](../../scripts/demo_intent_to_deploy.py)
- [Smoke test](../../scripts/integration/smoke_intent_to_deploy.py)
- [Wizard runbook](alfred-wizard.md)
