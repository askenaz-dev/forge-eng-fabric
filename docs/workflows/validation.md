# Phase 5 validation playbook

Tasks 13.1–13.3 are operational sign-off activities run with pilot
Workspaces. This page captures the scripted steps and where to record
evidence so the change can be archived.

## 13.1 Workspace pilot — design / eval / publish / install

Pilot Workspace `ws-pilot-A` (Tenant `tenant-acme`) authors a custom
workflow `refine-and-pr` and exercises every gate.

```bash
# 1. Create the workflow record
curl -X POST $REGISTRY/v1/workflows -d @- <<'EOF'
{ "id": "refine-and-pr", "tenant_id": "tenant-acme",
  "workspace_id": "ws-pilot-A", "name": "Refine and Open PR",
  "visibility": "workspace" }
EOF

# 2. Publish v1.0.0 from the editor (see docs/workflows/editor.md)

# 3. Register the eval dataset
curl -X POST $EVAL/v1/datasets -d @forge-workflows/eval-suites/release-train.yaml

# 4. Run regression and confirm `outcome=passed`
curl -X POST $EVAL/v1/runs/regression -d '{...}'

# 5. Promote to tenant — Tenant admin approves via Approvals Inbox

# 6. Install in ws-pilot-B and execute end-to-end
curl -X POST $MARKETPLACE/v1/marketplace/install -d \
  '{"tenant_id":"tenant-acme","listing_id":"...","target_workspace_id":"ws-pilot-B"}'
```

**Evidence**: capture `workflow.published.v1`,
`workflow.installed_to_workspace.v1`, and `workflow.eval.run_completed.v1`
events from the platform bus.

## 13.2 Run the 3 forge-certified workflows in production

For each of `release-train`, `scaffold-and-deploy`, `incident-response`
(under [`forge-workflows/`](../../forge-workflows)):

1. Publish via the registry as `forge-certified` with eval pass +
   security review id.
2. Trigger one real execution from the runtime.
3. Verify metrics surface in the
   [Phase-5 Grafana dashboard](../../deploy/compose/grafana/dashboards/phase-5-workflows.json).
4. Verify per-asset metrics appear in the Asset detail Observability tab.

## 13.3 Sign-off

Required signatures on the change before archive:

- Platform Engineering
- Engineering leadership
- Two pilot Workspace owners (one author, one consumer)

Record sign-off evidence as `decision_log` entries on the OpenSpec change
[`phase-5-workflow-marketplace`](../../openspec/changes/phase-5-workflow-marketplace).
