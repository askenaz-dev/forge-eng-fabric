## Why

The Forge platform delivers on its "From Intent to Infrastructure" promise architecturally — Phases 0–6 are specced, services are scaffolded, and most capabilities have a baseline implementation. But ten concrete GAPs prevent the platform from being usable end-to-end by a non-technical stakeholder, deployable outside a developer laptop, or operable under a documented compliance posture. Closing these GAPs converts the platform from "specced + scaffolded" into "demonstrably operable for a Tenant onboarding a real Business Unit and shipping a real application".

## What Changes

- **Conversational intent capture (P0)**: Replace the current slash-command Alfred Console with an Intent Capture Wizard that holds a natural-language dialogue, asks dynamic follow-up questions, and progressively assembles a complete OpenSpec. Admin-configurable autonomy levels SHALL be selectable per OpenSpec at capture time.
- **Reference end-to-end flow (P0)**: Wire `sdlc-orchestrator → scaffolder → app-onboarding → ci → deploy-orchestrator` into a documented happy-path workflow. Add `make demo-intent-to-deploy` and an integration smoke test.
- **Hardware & sizing guidance (P1)**: Document RAM/CPU/disk/cost tiers (Local, Staging, Production) with disable-flags for heavy components and per-service GKE sizing.
- **Helm chart coverage (P1)**: Author Helm charts for the ~26 services missing one, plus umbrella chart `forge-platform`. Each chart SHALL include values per env, NetworkPolicy, ServiceMonitor, HPA and PDB.
- **Phase 0/1 sign-off closure (P1)**: Close remaining sign-off tasks and produce signed evidence files for Phases 0 and 1.
- **Federated-runtime Terraform tooling (P2)**: Add `make verify-runtime` validator for BYO and Provisioned runtimes; document Phase 3 enablement procedure.
- **Tenancy onboarding doc (P2)**: Author non-technical `docs/concepts/tenancy-model.md` explaining Tenant→BU→Workspace with isolation matrix, configuration patterns and cost model.
- **Phase 2–6 enablement and sign-offs (P2)**: Complete operational instructions, runbooks and signed evidence for Phases 2 through 6, replacing current placeholders in `docs/platform-enablement.md`.
- **Workflow visual editor decision + implementation (P3)**: Resolve the n8n vs Flowise vs build-own decision via ADR; implement the editor wired to `workflow-runtime` with the catalog of node types listed in `workflow-visual-editor`.
- **Data retention & lifecycle (P3)**: Author `docs/governance/data-retention.md` and implement enforcement (audit_event partitioning, GCS archival, Loki/Tempo/Langfuse TTLs) keyed on data classification.

## Capabilities

### New Capabilities

- `intent-capture-wizard`: Conversational front-end in the Portal that turns a non-technical user's free-text intent into a complete, validated OpenSpec via dynamic follow-up questions, autonomy/HITL controls, and a final preview-and-execute hand-off to the SDLC orchestrator.
- `intent-to-deploy-reference-flow`: Reference end-to-end orchestration (workflow definition + smoke test + runbook + Make target) that demonstrates a complete intent→deploy traversal across `alfred → sdlc-orchestrator → scaffolder → ci → deploy-orchestrator`, gated by HITL approval before production.
- `platform-sizing-guidance`: Documented and version-controlled hardware/cost guidance per environment tier (Local, Staging, Production) covering every required platform component and per-service requests/limits.
- `kubernetes-deployment-charts`: Complete Helm chart coverage — one chart per service in `services/` plus an umbrella chart `forge-platform` that orchestrates a full-platform install with per-environment values, NetworkPolicy, ServiceMonitor, HPA and PDB.
- `tenancy-onboarding-guide`: Business-friendly canonical document explaining the Tenant→BU→Workspace hierarchy with cloud-analogy mapping, isolation matrix, configuration patterns and cost-allocation model — accessible to Product/Business stakeholders.
- `phase-rollout-evidence`: Per-phase sign-off process and evidence artifacts covering both retroactive closure of Phases 0 and 1 and forward closure of Phases 2 through 6, including runbook completion, integration evidence, and named approver sign-off in `docs/governance/`.
- `data-retention-lifecycle`: Data-classification-driven retention and lifecycle policies with documented governance and runtime enforcement across audit, traces, RAG content, logs and metrics.

### Modified Capabilities

- `alfred-control-plane`: Add admin-configurable autonomy levels surfaced to the user during intent capture, and a wizard-driven dialogue mode that complements the existing slash-command mode. Existing slash-command interaction SHALL remain available.
- `openspec-backbone`: Support progressive draft state for OpenSpecs in construction, with field-level completeness reporting so the wizard knows what to ask next, and a single transactional commit when the draft is complete and validated.
- `forge-provisioned-runtime`: Add a runtime-onboarding verification capability (preflight + post-provision health checks exposed through tooling) and clarify federated-project IAM scope minimization.
- `byo-runtime-onboarding`: Add the same runtime-onboarding verification surface so Workspace owners can validate either onboarding mode through one consistent tool.
- `workflow-visual-editor`: Formalize the implementation choice (build-on-top-of-Flowise vs n8n vs custom) via ADR reference, and confirm the canonical node catalog the editor SHALL render.

## Impact

- **Code**:
  - `portal/src/app/alfred/`: replaced with conversational wizard UI; existing slash-command page kept under a new route.
  - `services/alfred/`: new dialogue-orchestration endpoints (`POST /v1/intent/start`, `POST /v1/intent/answer`, `POST /v1/intent/commit`).
  - `services/openspec/`: progressive-draft state machine + completeness API.
  - `services/sdlc-orchestrator/`: reference workflow definition + smoke test entrypoint.
  - `services/runtime-registry/`: verification API consumed by `make verify-runtime`.
  - `portal/src/app/workflows/`: visual editor implementation per ADR.
- **Infra**:
  - `infra/helm/`: ~26 new charts + `infra/helm/forge-platform/` umbrella.
  - `infra/terraform/modules/`: completion of `gke-cluster`, `cloud-run-service`, `cloud-sql`, `memorystore`, `artifact-registry`, `iam-delegated-permissions`.
  - `Makefile`: new targets `demo-intent-to-deploy`, `verify-runtime`.
- **Docs**:
  - New: `docs/concepts/tenancy-model.md`, `docs/runbooks/intent-to-deploy-demo.md`, `docs/governance/data-retention.md`, `docs/governance/phase-0-signoff.md`, `docs/governance/phase-1-signoff.md`, `docs/governance/adrs/0001-workflow-visual-editor.md`.
  - Updated: `docs/platform-enablement.md` (Hardware & Sizing section, Phase 3 enablement, Phases 2–6 sections), `docs/getting-started.md` (link to sizing).
- **Dependencies**: Possible introduction of Flowise or n8n upstream as a dependency depending on ADR outcome (Phase 3-9 scope only).
- **Audit / governance**: New retention enforcement jobs and OpenFGA scope updates for federated-project provisioning.
- **Out of scope (deferred to follow-up changes)**: Multi-cloud beyond GCP; chargeback billing integration with finance ERPs; full localization of the wizard beyond English/Spanish.
