# Runbook: `forge.reference.intent-to-infrastructure@1` Demo

**Workflow version:** 1.0.0  
**Last validated:** 2026-05-14  
**Make target:** `make demo-intent-to-infrastructure`  
**Spec:** `openspec/changes/sdlc-end-to-end`

---

## Prerequisites

| Requirement | How to verify |
|-------------|---------------|
| Local stack healthy | `make up` then `docker compose ps` — all services `healthy` |
| Minikube runtime registered | `forge list` shows a runtime with `provider=minikube` |
| Feature flags enabled per-tenant | `forge.workflow.intent_to_infrastructure.enabled=true`, `forge.sdlc.*.enabled=true`, `forge.sdlc.iac.enabled=true` |
| Python 3.11+ | `python --version` |

Enable the required feature flags for the local platform tenant:

```bash
curl -s -X PATCH http://localhost:8082/v1/tenants/local/feature-flags \
  -H "content-type: application/json" \
  -d '{
    "forge.workflow.intent_to_infrastructure.enabled": true,
    "forge.sdlc.architecture_skills.enabled": true,
    "forge.sdlc.design_skills.enabled": true,
    "forge.sdlc.qa_skills.enabled": true,
    "forge.sdlc.iac.enabled": true,
    "forge.healing.l1_l2.enabled": true
  }'
```

---

## Running the demo

```bash
make demo-intent-to-infrastructure
```

Optional environment overrides:

| Variable | Default | Description |
|----------|---------|-------------|
| `ALFRED_URL` | `http://localhost:8090` | Alfred service endpoint |
| `WORKFLOW_RUNTIME_URL` | `http://localhost:8095` | Workflow runtime endpoint |
| `APPROVALS_URL` | `http://localhost:8105` | Approvals service endpoint |
| `APPLICATION_URL` | `http://localhost:8095` | Application service endpoint |

The script:
1. Submits a canned intent via Alfred.
2. Commits the intent to produce an `openspec_id` and `app_id`.
3. Starts `forge.reference.intent-to-infrastructure@1` with `include=[iac, observability]` and all targets set to `required`.
4. Polls the workflow run, auto-approving every HITL gate (using `X-Forge-Demo-Auto-Approve: true`).
5. Writes a JSON report to `build/demo-intent-to-infrastructure/<timestamp>.json`.

---

## Expected step sequence

The demo asserts the following milestones are emitted in order:

| Step | Event |
|------|-------|
| commit-intent | `intent.committed.v1` |
| scaffold-repo | `repo.scaffolded.v1` |
| propose-adr | `sdlc.adr.proposed.v1` |
| generate-api-contract | `sdlc.api_contract.proposed.v1` |
| propose-data-model | `sdlc.data_model.proposed.v1` |
| lightweight-threat-model | `sdlc.threat_model.completed.v1` |
| generate-ui-blueprint | `sdlc.ui_blueprint.proposed.v1` |
| generate-component-stubs | `sdlc.component_stubs.committed.v1` |
| accessibility-audit | `sdlc.accessibility_audit.completed.v1` |
| open-development-pr | `pr.opened.v1` |
| generate-test-plan | `sdlc.test_plan.proposed.v1` |
| security-review (HITL) | `workflow.paused_for_approval.v1` |
| generate-terraform | `sdlc.iac.generated.v1` |
| validate-iac | `sdlc.iac.validated.v1` |
| apply-iac | `sdlc.iac.applied.v1` |
| deploy-staging | `deploy.completed.v1{env=staging}` |
| approve-prod-deploy (HITL) | `workflow.paused_for_approval.v1` |
| deploy-prod | `deploy.completed.v1{env=prod}` |
| configure-slo | `sre.slo.configured.v1` |
| provision-dashboards | `observability.dashboards.provisioned.v1` |

---

## Reading the JSON report

```json
{
  "workflow": "forge.reference.intent-to-infrastructure@1",
  "correlation_id": "<uuid>",
  "generated_at": "<ISO-8601>",
  "success": true,
  "steps": [
    { "step": "start-intent", "at": "...", "outcome": "ok", "intent_id": "..." },
    ...
  ],
  "deploy_url": "https://app-1.staging.local/",
  "observability_urls": ["https://grafana.local/d/..."]
}
```

---

## Common failure modes

### `start-intent` fails with 503

Alfred is not yet healthy. Wait 30 seconds and retry, or check `docker compose logs alfred`.

### `workflow-timeout` after polling

The workflow runtime is stuck. Check `docker compose logs workflow-runtime`. Most common cause: a HITL gate waiting for approval that was not auto-approved (ensure `X-Forge-Demo-Auto-Approve: true` reaches the approvals service).

### `validate-iac` returns `status=failed`

The generated IaC bundle failed conftest validation. Inspect the `iac_validation_report` in the step output and check the policy bundle at `policies/iac/`. Common violation: missing `NetworkPolicy` in generated Helm values.

### Feature flag not enabled

The workflow silently skips phases whose feature flags are off. Check with:

```bash
curl http://localhost:8082/v1/tenants/local/feature-flags | jq .
```

---

## Rollback

1. Delete the workflow run: `curl -X DELETE http://localhost:8095/v1/runs/<run_id>`
2. Archive the App created by the demo if needed: `curl -X POST http://localhost:8095/v1/apps/<app_id>:archive`
3. The IaC PR opened by `apply-iac` can be closed without merging — no infra changes are applied until the PR is merged.

---

## Supported make target flags

None currently. All configuration is via environment variables (see table above).

---

## Updating this runbook

Update this file whenever a step is added or removed from the workflow YAML at
`forge-workflows/reference/intent-to-infrastructure@1.yaml`. Include the workflow version
and the date of last validation at the top of this file.
