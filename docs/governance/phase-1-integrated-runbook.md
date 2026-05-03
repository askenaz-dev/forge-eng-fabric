# Phase 1 Integrated Validation Runbook

Purpose
- Provide a reproducible, step-by-step runbook to validate the remaining Phase‑1 exit criteria that require an integrated environment and SDLC sign-off (tasks 13.5, 13.6, 13.8).

Scope
- This runbook assumes the platform services for Phase‑1 are deployed to a staging environment and reachable via HTTP(S). It walks engineers/QA through verifying:
  - Promotion gating by eval scores
  - Blocking of `in_review` assets in production‑relevant flows
  - End‑to‑end correlation (correlation_id) across Alfred decision log, Langfuse traces and OpenTelemetry (Tempo)
  - Evidence collection for SDLC sign-off

Prerequisites
- Access to staging environment with the following services deployed and reachable:
  - Alfred service (Alfred Console / API)
  - OpenSpec service (openspec API)
  - Registry service (asset lifecycle)
  - Policy Engine
  - Approvals service
  - Prompt Registry / Prompt Template service
  - MCP servers (GitHub, Jira, Confluence) or test doubles
  - LiteLLM gateway
  - RAG (Milvus) reachable by Alfred
  - Langfuse (or equivalent) for AI observability
  - OpenTelemetry backend (Tempo) for traces

- Credentials and secrets (stored securely; DO NOT commit secrets to git):
  - KEYCLOAK_CLIENT_ID / CLIENT_SECRET or service account for Alfred
  - OPENFGA_API_ENDPOINT / OPENFGA_TOKEN
  - JIRA_BASE / JIRA_TOKEN (or test Jira instance)
  - CONFLUENCE_BASE / CONFLUENCE_TOKEN
  - LANGFUSE_API_KEY (or equivalent)
  - ALFRED_API_URL (e.g., https://alfred.staging.example)
  - OPENSPEC_API_URL
  - REGISTRY_API_URL
  - POLICY_API_URL
  - APPROVALS_API_URL

High‑level validation steps
1. Health checks
   - Confirm the endpoints are reachable and healthy.
   - Example: curl -sSf "$ALFRED_API_URL/health" && echo OK

2. Prepare test workspace and principal
   - Create a test Workspace (via Portal or openspec/registry API).
   - Create or identify a test principal (service account) for Alfred with delegated permissions.

3. Grant delegated permission to Alfred (automated or UI)
   - POST to approvals or permissions API to grant Alfred action_class `openspec:write` scoped to the test workspace for 7 days.
   - Verify OpenFGA tuple or permission query returns allowed for Alfred.

4. Create a low‑eval OpenSpec and verify promotion rejection
   - Create or import an OpenSpec asset with eval scores below the threshold for T1.
   - Attempt to promote the asset to `approved` via registry API.
   - Expected: API rejects promotion with a list of failing eval dimensions.

5. Run Alfred intent that creates OpenSpec and invokes Skills
   - Call Alfred: POST /v1/intents (see example payload below) with correlation_id
   - Wait for Alfred to process the intent and inspect decision log via GET /v1/decisions?correlation_id=<id>
   - Verify decision log contains tool invocations for mcp:openspec.create, skill:create-user-stories, skill:generate-test-cases and that all have outcome `succeeded`.

6. Verify Langfuse traces and Tempo correlation
   - Search Langfuse for the correlation_id; verify presence of model call trace and tool call spans.
   - Verify Tempo/OpenTelemetry traces link to the same correlation_id.

7. Test promotion with passing evals
   - Run the eval harness to produce passing eval scores for the asset (or replace evals in the registry if allowed for testing).
   - Attempt promotion to `approved`; expected: success and lifecycle transition event emitted.

8. Verify production gating for `in_review` assets
   - Mark an asset as `in_review` and attempt to invoke it in a production flow (metadata env=prod) via Alfred.
   - Expected: invocation blocked by policy engine and an audit event created.

9. Collect evidence for SDLC sign‑off
   - Decision logs (JSON) for the runs (include correlation_id).
   - Langfuse traces (trace ids and screenshots/exports).
   - Tempo/OpenTelemetry traces correlated to correlation_id.
   - Registry events showing lifecycle transitions and eval scores.
   - Approval records and grant records from the Portal or Approvals API.
   - Attach these to docs/governance/phase-1-signoff.md and notify SDLC Team.

Example Alfred intent payload (POST /v1/intents)
```
{
  "actor": "alice",
  "workspace_id": "<workspace-uuid>",
  "intent": "Validate Phase 1 E2E from Alfred Console",
  "correlation_id": "phase-1-integrated-<random>",
  "openspec_id": "phase-1-integrated",
  "metadata": {"env": "dev"}
}
```

Automated checks
- The companion script (scripts/integration/run_phase1_integrated_checks.ps1) automates health checks, permission grant, intent submission, polling for decisions and basic verification of outcomes. It requires the environment variables listed above.

When to stop and escalate
- If any step fails due to missing infrastructure (e.g., Langfuse not configured), stop and record the blocker.
- If policy decisions differ from expected, collect policy evaluation outputs and escalate to Security/Policy owners.

Sign-off
- Prepare a short evidence bundle (JSON logs + trace ids) and update docs/governance/phase-1-signoff.md with the evidence links. SDLC Team needs to confirm sign-off in that doc.

Appendix: common API endpoints (examples)
- Alfred intents: POST $ALFRED_API_URL/v1/intents
- Decisions: GET $ALFRED_API_URL/v1/decisions?correlation_id=<id>
- Registry promote: POST $REGISTRY_API_URL/v1/assets/{asset_id}/lifecycle/promote
- Permission grant (example): POST $APPROVALS_API_URL/v1/grants

Notes
- This runbook intentionally uses conservative, reproducible steps: prefer API interactions to UI clicks for automation. Replace placeholders with your staging endpoints and secure credentials.
