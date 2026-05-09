## Context

Forge has reached the end of its initial six-phase roadmap with all archived spec changes (`phase-0-foundations` through `phase-6-autonomous-ops`) merged. The README, however, accurately states that local-first foundations are implemented "with remaining sign-off/deferred infra items tracked in OpenSpec" and Phases 2–6 are still in "implementation-pending" status. The Alfred Console at [portal/src/app/alfred/page.tsx](portal/src/app/alfred/page.tsx) is a slash-command interface, suitable for power users but unusable by the non-technical stakeholders the platform exists to serve. Helm coverage is partial (7 of ~33 services). Terraform modules are stubbed. The visual workflow editor capability exists in spec (`workflow-visual-editor`) but the ADR choice (n8n / Flowise / build-own) is unresolved. Data retention has an open question in the bootstrap design doc.

The ten GAPs in the proposal are interrelated but separable. They naturally split into a documentation track (sizing, tenancy, retention, sign-offs), a deployment-tooling track (Helm, Terraform, runtime verification), and a product-experience track (intent wizard, reference flow, visual editor). All three tracks are required to convert the architectural promise into an operable product.

Constraints we accept up front:
- **No regression of slash-command Alfred**. Power users rely on it; the wizard is additive.
- **No new runtime languages**. Wizard backend stays in Python+FastAPI alongside existing Alfred service. Portal stays in Next.js.
- **OpenSpec progressive draft semantics must remain compatible** with the existing `openspec_id` model so partially-built drafts can be persisted, listed, abandoned, or completed without conflicting with already-committed OpenSpecs.
- **Helm chart authoring must be mechanically uniform** so reviewers can read all 26 charts confidently — we will use a single template per service flavor (HTTP service, worker, cron job).
- **Federated GCP IAM** is the only multi-tenant cloud model in scope; AWS/Azure equivalents are deferred.
- **Phase sign-off evidence is contractually load-bearing** — the platform's claim that "from intent to infrastructure" works depends on these documents being authentic, dated, and attributable.

## Goals / Non-Goals

**Goals:**
- A non-technical user can complete an intent→deployed application loop with no slash commands, guided entirely by the wizard and HITL gates.
- An operator can install the entire platform on a single Kubernetes cluster via `helm install forge-platform infra/helm/forge-platform` with appropriate values.
- A platform admin can run `make verify-runtime WORKSPACE=<id> RUNTIME=<gke|cloudrun|minikube>` and receive an actionable pass/fail report.
- Phases 0–6 each have a single linkable sign-off document with completion evidence.
- The visual workflow editor renders nodes from the live registry catalog and persists workflows in canonical AST.
- Data retention is enforced automatically per data classification.

**Non-Goals:**
- Replacing or deprecating the slash-command Alfred Console.
- Multi-cloud (AWS/Azure) Terraform modules.
- Localizing the wizard beyond English and Spanish.
- A drag-and-drop UI for wizard authoring (admins author follow-up question templates as YAML/JSON).
- A general-purpose "deploy any app to any runtime" abstraction beyond the three runtimes already in scope (GKE, Cloud Run, Minikube).
- Chargeback integration with external finance/ERP systems (showback only; chargeback enabled in `finops-recommendations` future work).
- A new agent identity model — Alfred remains the single Control Plane Agent.

## Decisions

### D1. Wizard runs as a thin Portal client over an Alfred dialogue API

**Decision**: Implement the wizard UX in the Portal (`portal/src/app/alfred/wizard/`). The wizard makes calls to a new dialogue surface on the Alfred service: `POST /v1/intent/start`, `POST /v1/intent/answer`, `GET /v1/intent/{draft_id}`, `POST /v1/intent/commit`. Alfred holds dialogue state in Postgres and uses LiteLLM for the question-generation prompt and field-extraction prompt.

**Rationale**: Alfred already owns LLM access via LiteLLM, knowledge-base retrieval, and policy evaluation. Putting the dialogue logic anywhere else would duplicate that surface. The Portal stays presentation-only, which matches the established pattern (`portal/` calls `services/*` over typed APIs).

**Alternatives considered**:
- *Standalone wizard service*: rejected — duplicates LiteLLM, OpenSpec, and policy integrations Alfred already has.
- *Pure client-side LLM calls*: rejected — bypasses LiteLLM gateway, breaks audit and budget tracking.
- *Reusing the existing slash-command parser*: rejected — slash commands assume the user knows the schema; the wizard's premise is that they don't.

### D2. Progressive OpenSpec drafts via a `draft` lifecycle state

**Decision**: Extend `openspec-backbone` with a draft state machine. A draft has `status ∈ {drafting, validating, committed, abandoned}`, the same `openspec_id` namespace as committed OpenSpecs, and a per-field `completeness` map. Drafts are not visible to non-author principals and do not block production-relevant actions until committed. The `committed` transition is atomic — when validation passes, the OpenSpec becomes a normal first-class OpenSpec.

**Rationale**: Wizard interactions are inherently incremental; rejecting partial OpenSpecs at every keystroke would defeat the conversational flow. Keeping drafts in the same namespace avoids a parallel data model and lets the wizard's "Ejecutar SDLC" button operate on the standard OpenSpec.

**Alternatives considered**:
- *In-memory draft only*: rejected — loses progress on tab close, prevents resumption.
- *Separate `intent_draft` table with conversion at commit*: rejected — duplicates the validation surface, risks divergence.

### D3. Autonomy levels are workspace-scoped admin presets, instance-overridable

**Decision**: Workspace admins define a set of named autonomy presets (e.g., `full-autonomy`, `staging-only`, `manual-prod`). Each preset is an `autonomy_policy` block compatible with `openspec-backbone`. The wizard surfaces presets as a dropdown plus an "advanced" expandable view that lets the user override individual action classes per OpenSpec, subject to maximum-autonomy ceilings the admin sets.

**Rationale**: The user's stated requirement is admin-controlled autonomy with per-instance adjustability. A preset model keeps the wizard friendly while preserving the existing `autonomy_policy` schema and policy enforcement.

**Alternatives considered**:
- *Single global setting*: rejected — too coarse for mixed-criticality workloads.
- *Free-form per-OpenSpec policy with no ceilings*: rejected — undermines admin governance.

### D4. Reference flow is a workflow registered in `workflow-runtime`, not a script

**Decision**: Implement the intent-to-deploy reference flow as a versioned workflow definition in `workflow-runtime` named `forge.reference.intent-to-deploy@1`. The workflow is the source of truth; `make demo-intent-to-deploy` triggers it via the Alfred API; the smoke test asserts execution-trace milestones; the runbook documents the human steps and HITL gates.

**Rationale**: The platform's promise is "workflows, not scripts." The reference flow MUST itself be a workflow, both as dogfooding and so customers can fork it.

**Alternatives considered**:
- *Bash script orchestrating CLIs*: rejected — circumvents the policy/audit/eval pipeline that production flows go through.
- *Hard-coded test fixture*: rejected — same as above; also drifts from runtime code.

### D5. Helm chart authoring uses three flavor templates

**Decision**: All ~26 charts derive from one of three templates: `service-http` (FastAPI/Next.js services exposed via Service+Ingress), `service-worker` (event consumers without inbound HTTP), `service-cron` (scheduled jobs). Each template includes baseline `Deployment`, `Service` (HTTP only), `ServiceMonitor`, `NetworkPolicy`, `HorizontalPodAutoscaler`, `PodDisruptionBudget` and `values.yaml` with env overlay support.

**Rationale**: Mechanical uniformity makes review tractable. Differences between services are values, not template logic.

**Alternatives considered**:
- *Per-service hand-authored charts*: rejected — review burden, inconsistent posture.
- *Single library chart consumed via subchart*: rejected — debugging values inheritance is harder for new contributors than reading three flat templates.

### D6. Umbrella chart `forge-platform` composes via `dependencies:` not `tpl` includes

**Decision**: The umbrella chart references each service chart as a dependency in `Chart.yaml`. Tier presets (`small`, `medium`, `large`) are values files at the umbrella level; per-service overrides bubble down through standard Helm subchart values.

**Rationale**: Native Helm composition; charts remain installable individually for component-level testing.

### D7. `make verify-runtime` consumes a Runtime Verifier API on `runtime-registry`

**Decision**: `services/runtime-registry/` exposes `POST /v1/runtimes/{id}/verify` returning a structured report `{ workspace_id, runtime_id, type, checks: [{name, status, evidence, remediation}] }`. The Make target wraps this API call and renders human-readable output. The verifier checks: connectivity, IAM scopes against expected federated minimums, image-pull from the Workspace's Artifact Registry, network egress to required endpoints, observability collector reachability.

**Rationale**: Encoding checks in a service (not a shell script) keeps them reusable from CI, the Portal, and Alfred. Uniform check-list across BYO and Provisioned modes satisfies REQ-GAP-05.

### D8. Tenancy doc is treated as canonical reference, not marketing collateral

**Decision**: `docs/concepts/tenancy-model.md` is owned by Platform Architecture, reviewed by Security and Finance, and kept stable across releases. It is linked from the Portal's "About this Workspace" panel and from `docs/platform-enablement.md`. It contains: hierarchy diagram, GCP-analogy mapping (Tenant ≈ Org/Billing root, BU ≈ Folder, Workspace ≈ Project), isolation matrix, configuration patterns (one cluster per BU vs per Workspace vs shared with namespaces), cost model (showback default, chargeback open-question kept).

**Rationale**: Stakeholders churn faster than the model does — a stable canonical doc beats embedding the model in onboarding decks.

### D9. Phase sign-off evidence is a structured Markdown file with required sections

**Decision**: Each `docs/governance/phase-{n}-signoff.md` contains: scope summary, exit criteria checklist (each linked to the spec in the archive), evidence links (PRs, runbook executions, eval reports), known deferred items, signing approvers (name + role + date). The file is cryptographically signed via `git tag -s` on the corresponding archive commit.

**Rationale**: Sign-off is contractual; it must be auditable post-hoc. Markdown + signed git tag is sufficient and avoids new infrastructure.

### D10. Visual editor — embed Flowise

**Decision**: Embed Flowise (LGPL-compatible) as the visual editor host, contributing back our node catalog adapter. Persist canonical AST in `workflow-registry`; the editor reads/writes via a thin adapter that translates between Flowise's format and our AST.

**Rationale**: Flowise is closer to LLM-native node primitives we need (LLM, MCP, Agent, Prompt Template) than n8n's automation-first taxonomy. Build-own is rejected — the engineering cost (estimated at 3+ engineer-quarters) does not pay for itself when an open-source option exists. n8n is rejected because forking introduces incompatibility with upstream and its node model maps poorly to Skills/MCPs.

**Alternatives considered**:
- *n8n fork*: rejected per above.
- *Build own (React Flow + custom node SDK)*: rejected per above.
- *No visual editor, YAML only*: rejected — fails the non-technical user promise.

### D11. Data retention enforcement runs as scheduled jobs in `audit-service` and `observability` operators

**Decision**: Retention is enforced by:
- A nightly cron job in `audit-service` that partitions `audit_event` by month, archives partitions older than the classification's retention to GCS, and drops them from Postgres.
- Loki and Tempo retention configured via Helm values per environment tier.
- Langfuse data retention configured via its native retention API, set per Workspace.
- RAG content (Milvus) governed by ingestion-time TTL plus a periodic re-evaluation against the source's classification.

**Rationale**: Each system enforces its own TTL; centralized retention "policing" would be brittle. The governance doc lists the source of truth per data type.

## Risks / Trade-offs

- **Wizard LLM cost** → Mitigation: cache common follow-up question templates per domain, only call LiteLLM for novel branches; cap turns per draft (default 12).
- **Progressive draft enables abandoned-draft accumulation** → Mitigation: 14-day expiry on drafts in `drafting` state; admin dashboard surfaces stale drafts.
- **Helm template homogeneity may force awkward values for outlier services (e.g., Kafka-bound workers)** → Mitigation: allow chart-local overrides via a documented escape hatch per template; review the three flavors after the first three charts to validate fit.
- **Flowise upstream LGPL terms require contributing changes** → Mitigation: keep adaptations isolated in an adapter package and contribute them upstream; track license inventory in `docs/governance/licenses.md`.
- **Retention enforcement could prematurely delete data needed for a forensic investigation** → Mitigation: legal-hold mechanism that pauses retention for tagged objects; documented in the retention doc.
- **`make verify-runtime` cannot fully simulate production policy without test traffic** → Mitigation: scope the verifier to declared invariants (connectivity, IAM, image-pull); integration assertions happen in the reference flow.
- **Phase sign-offs being signed retroactively** for Phases 0/1 may surface gaps during evidence-gathering that block sign-off → Mitigation: explicitly allow sign-off-with-deferred-items, listing each deferred item as a follow-up change.
- **Helm chart count (~26 new charts) is the largest single contribution in this change** → Mitigation: bulk-author with the three templates; review per-flavor batches not per-chart.

## Migration Plan

This change is additive in spec terms — no existing OpenSpecs become invalid. Rollout sequence:

1. **Wave 1 — Documentation foundations (low-risk, unblocks reviewers)**: tenancy-model.md, sizing tiers in platform-enablement.md, data-retention.md, phase-0/1 sign-offs.
2. **Wave 2 — Helm chart batch + verifier API**: ship charts in three batches (HTTP services, workers, cron jobs); introduce `runtime-registry` verifier endpoint behind a feature flag; expose `make verify-runtime`.
3. **Wave 3 — Progressive OpenSpec draft + Alfred dialogue API + Wizard UI**: ship the API first behind a feature flag, then the Portal wizard; keep slash-command UI live throughout.
4. **Wave 4 — Reference flow + demo Make target**: register `forge.reference.intent-to-deploy@1` workflow; add smoke test in CI; publish runbook.
5. **Wave 5 — Visual workflow editor**: ADR landed, Flowise adapter shipped, editor wired to `workflow-runtime`.
6. **Wave 6 — Phase 2–6 sign-offs**: with prior waves available, each phase's enablement section is completed and signed in turn.

Rollback strategy:
- Wizard is feature-flagged at the Portal route level (`?wizard=1`) and at the Alfred service level (`ALFRED_DIALOGUE_API=enabled`). Disabling the flag returns users to the slash-command UI.
- Helm charts are independently installable; rolling back the umbrella chart leaves individual installs intact.
- Retention jobs require an explicit `ENFORCE_RETENTION=true` flag per environment; default is dry-run mode that logs intended deletions for two weeks before enforcement.

## Open Questions

1. **Wizard authoring of follow-up question templates**: are templates seeded by Platform team only, or can Workspace admins add domain-specific templates? (Default in this change: Platform-only; admin-extensible deferred.)
2. **Chargeback vs showback** for cluster/runtime cost: this change documents showback; chargeback wiring to external billing is deferred to a `finops-chargeback` future change.
3. **Sign-off approver roles** for Phases 2–6: who must sign for F4 (SDLC orchestration)? Default proposal: SDLC Lead + Platform Architect + Security Lead. To be confirmed during F4 sign-off task.
4. **Flowise upstream version pinning**: pinned vs floating? Default: pin to a known-good release with quarterly upgrade tasks tracked.
5. **Tenant-level retention overrides**: can a Tenant lengthen retention beyond the platform default for legal-hold cases? Default: yes, with audit. Tightening below platform minimums is not allowed.
