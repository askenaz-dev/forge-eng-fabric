## Context

The SDLC orchestration in Forge is shaped like a fan: at the top sits Alfred (control plane), driving the `sdlc-orchestrator` which is supposed to dispatch into nine phase capabilities (Product, Architecture, Design, Development, QA, Security, DevOps, SRE, FinOps). Each phase exposes 2–4 skills and a set of gates. Today:

- `sdlc-architecture`, `sdlc-design` and `sdlc-qa` are **declared** with their skill names and gates but the skill implementations are missing. The orchestrator can't actually invoke them, so runs that need an API contract / threat model / UI blueprint / generated tests simply skip those steps.
- `healing-engine` is implemented at L3–L5 (the action-execution levels with HITL approval, autonomous and rollback). L1 (Notify) and L2 (Suggest) — the *upstream* levels where the platform detects a problem and *proposes a fix without executing* — are gappy: the detection signal is there (`diagnosis-pipeline`) but there's no wiring that turns a detected anomaly into a propose-fix that lands in an inbox.
- `sdlc-infrastructure` doesn't exist as a spec. IaC generation happens ad-hoc inside individual repos; the platform has no opinion about Terraform/Helm wiring.

This change fills those five gaps and binds them into a new reference workflow `forge.reference.intent-to-infrastructure@1` that goes deeper than the existing `forge.reference.intent-to-deploy@1`. The current intent-to-deploy stops at "deploy to staging+prod" — the new workflow goes through *every* phase including IaC and observability with App-declared opt-ins.

We landed two prerequisites in this Phase 5 bundle: `app-first-class-entity` (so each run is anchored to an App, with an OpenFGA scope and a target matrix) and `design-system-catalog` (so the design step can pick the App's Design System when generating component stubs). With those in place, the SDLC end-to-end work is mostly *plumbing + skill implementation + workflow definition*, not net-new architecture.

Constraints:
- Skills MUST be invokable through the existing skill-gateway and the `mcp-and-skills` runtime contract — no parallel calling pattern.
- Eval suites MUST exist for every skill before it leaves T0; the existing eval harness handles this.
- The IaC apply step MUST be PR-driven; no skill SHALL run `terraform apply` directly against tenant infrastructure.
- L1/L2 healing MUST not call any action that exists in the L3+ catalog without going through the inbox.

Stakeholders: SDLC team (capability owners), Alfred team (orchestration), Workflow team (DSL + runtime extension), Healing team (L1/L2 wiring), Infra team (Terraform modules + Helm templates), portal team (Approvals Inbox renderer for L2 fixes).

## Goals / Non-Goals

**Goals:**
- Implement the missing skills for `sdlc-design`, `sdlc-architecture`, `sdlc-qa` and wire them into the orchestrator with their gates.
- Implement L1 detect + L2 propose-fix for `healing-engine` covering the existing healing-action-catalog actions.
- Introduce `sdlc-infrastructure` as a new capability: Terraform module generation, Helm values generation, validation (plan/lint/conftest), and PR-driven apply.
- Ship `forge.reference.intent-to-infrastructure@1` as the new canonical reference workflow with an App-level `targets:` opt-in matrix.
- Keep `forge.reference.intent-to-deploy@1` working unchanged as the smaller default.

**Non-Goals:**
- Multi-region infra topology, blue/green or canary deploys (those live under `deployment-policies` and are out of scope).
- A net-new specialized agent for any of the new capabilities — skills are sufficient and avoid duplicating Alfred's coordination layer.
- L4/L5 healing improvements — those are stable; this change only fills L1/L2.
- AWS-specific differentiators (e.g., AWS Lambda recipes). The Terraform modules cover the three providers the platform supports for runtimes, no provider-favouring features.
- Authoring brand new design tokens. The design step consumes the App's Design System via `design-system-catalog` and emits component stubs that follow it.

## Decisions

### Decision 1 — Skills, not new specialized agents

**Choice**: every capability addition (UI blueprint, component stubs, a11y audit, API contract, data model, threat model, test plan, e2e tests, triage failures, IaC gen/validate/apply, healing detect, healing propose-fix) is a **skill** registered in the Asset Registry with an eval suite. None of them is a specialized agent.

**Why**: skills inherit the existing trust model, the existing eval harness, the existing approval/audit pipeline. A specialized agent would force us to build another coordination layer on top of Alfred. The cost of "one more skill" is small; the cost of "one more agent" is operational.

**Alternatives considered**: *one specialized "Architect Agent"* — rejected, Alfred already coordinates phases.

### Decision 2 — Wire reactive QA triage from `ci.failed.v1`

**Choice**: when a CI run for an App's repo fails (`ci.failed.v1` event emitted by the CI integration), the platform automatically invokes `triage-test-failures` against the failing run. The skill produces a structured triage report (top hypotheses, affected files, proposed minimal patch). The orchestrator then posts a PR comment with the report and (when the App's target for `qa` is `required` or `autonomous`) auto-opens a fix PR.

**Why**: most QA value comes from the *feedback loop*, not from the first test-generation pass. Wiring the reactive triage closes the loop without giving Alfred a new pager.

**Alternatives considered**:
- *Only run triage on operator request*. Rejected — half the value is removed.
- *Auto-open the fix PR always*. Rejected — needs the App's `qa` target to be `required` or `autonomous`; an `optional` target only posts the report.

### Decision 3 — Healing L1 is notify-only; L2 is propose-fix-only

**Choice**: L1 sits at the *detect* boundary. Every detection (anomaly in metrics, CI red, deploy failure, alert threshold crossed) emits `healing.detected.v1` with rich HITL context (the diagnosis report, candidate hypotheses, candidate actions, blast-radius estimate). No action is taken. L2 sits at the *propose-fix* boundary. The engine picks the highest-confidence hypothesis, generates a *proposed fix* (code diff or config change) and routes it through the **Approvals Inbox** as a new approval card with the diff rendered inline. Approving the card transitions to L3 execution (existing flow). Rejecting it records the reason and closes the loop.

**Why**: clean level separation. The user explicitly asked for L1+L2 detect + propose-fix with HITL — we treat L1 as "tell me" and L2 as "tell me + suggest the patch", never as an executor. The L3+ executors are unchanged.

### Decision 4 — `sdlc-infrastructure` apply is PR-driven, never direct

**Choice**: every `apply-iac` invocation produces a PR against the App's infra repo (`infra/<app-slug>` by convention) containing the generated Terraform / Helm changes, a `terraform plan` output as a checked-in comment, the `helm template` output and the conftest results. Merging the PR triggers an `apply-iac` runner (the GitOps pipeline) that executes `terraform apply` and `helm upgrade --install`. The skill itself NEVER runs the apply.

**Why**: aligns with the platform-wide GitOps stance, makes every infra change reviewable in git, eliminates the "skill has prod creds" footgun.

### Decision 5 — Workflow `targets:` lives on the App, not on the spec

**Choice**: the App entity carries a `targets` JSONB map declaring which SDLC phases are `required` / `optional` / `opt-in` / `skipped`. The reference workflow consults this map at every step. The OpenSpec MAY override the App-level targets per-spec, but the App-level setting is the default.

**Why**: the App is the natural anchor for delivery policy. Most teams want to set the targets once per App (e.g., "this internal tool doesn't need a threat model"); per-spec overrides are an escape hatch.

**Allowed values per phase**:
- `required` — the step runs and the run fails if the step fails
- `optional` — the step runs; failure produces a warning but does not fail the workflow
- `opt-in` — the step runs only when the operator explicitly requests it (a flag on workflow start)
- `skipped` — the step is removed from the plan entirely

**Defaults** (set at App creation):
```
architect: required
design: optional
development: required
qa: required
security: required
devops: required
sre: optional
finops: opt-in
iac: opt-in
observability: opt-in
```

### Decision 6 — New reference workflow, old one preserved

**Choice**: `forge.reference.intent-to-infrastructure@1` is a brand-new workflow registered alongside `forge.reference.intent-to-deploy@1`. The old workflow remains the smaller default (intent → deploy). The new workflow is the canonical "deep" run and is what the Friendly view's "Nueva App" card kicks off by default.

**Why**: zero risk to existing pilots that rely on `intent-to-deploy`. Operators can pick which workflow to run; the Friendly view picks the deep one because Friendly users want "make it work end to end".

### Decision 7 — Eval suite is the gating contract for promoting any new skill

**Choice**: every new skill ships with an eval suite (≥30 graded fixtures), a baseline score and a promotion threshold. The skill SHALL NOT move beyond `trust_level=T1` without meeting the promotion threshold for the phase: T2 requires ≥0.80, T3 requires ≥0.90 for the in-distribution suite plus zero critical failures on the safety suite.

**Why**: keeps quality measurable and consistent with existing skill-gateway contracts.

## Risks / Trade-offs

- **[Risk] Skill implementations regress at scale**. The generated artefacts (API contracts, Terraform modules) are LLM-driven and may drift in quality. → Mitigation: per-skill eval suites + canary fixtures run on every commit to the skill server.
- **[Risk] Reactive triage spam in the App's PR comment thread**. → Mitigation: rate-limit `triage-test-failures` to one report per 10 minutes per PR; collapse multiple failing runs into a single updated comment.
- **[Risk] PR-driven IaC apply slows down emergency hotfixes**. → Mitigation: a `break_glass=true` flag bypasses the PR for a specifically-flagged change with elevated approval (security-admin + platform-admin pair), recorded in audit.
- **[Risk] L2 propose-fix produces a wrong patch that the operator accepts blindly**. → Mitigation: the patch is always reviewable in the Approvals Inbox with file-level diff rendering and the diagnosis citations; approval requires explicit OK on the file selection; the eval suite for `propose-fix` includes adversarial fixtures with red-team patches.
- **[Risk] `targets:` map gets out of sync with reality** (e.g., App marks `iac: skipped` but actually needs IaC for a new service it onboards). → Mitigation: the workflow surfaces a runtime warning when a step that would otherwise be needed is skipped (e.g., new service onboarded but `iac: skipped`); App owners get a notification.
- **[Risk] The new workflow + new skills + new IaC capability blow up the audit volume**. → Mitigation: every new event is keyed by `correlation_id` and indexed; dashboards filter by App; retention policies match existing classes.
- **[Trade-off] Skills are LLM-bound and cost-bound**. → Acceptable: LiteLLM budgets already cap tenants; the new skills inherit the existing budget controls.
- **[Trade-off] The new workflow is larger than the old one**. → Acceptable: opt-in via `targets:` means it can collapse to the shape of the old one when needed.

## Migration Plan

1. **M0 — Infrastructure**: ship the new skill services (`sdlc-architecture-skills`, `sdlc-design-skills`, `sdlc-qa-skills`, `sdlc-iac`) behind feature flags `forge.sdlc.<phase>_skills.enabled` (per-tenant). Add `App.targets` column with defaults.
2. **M1 — Skill T1 publication**: publish each new skill at T0/T1 in the platform tenant, with eval suites running, but not yet wired into the orchestrator.
3. **M2 — Orchestrator wiring**: wire the orchestrator to invoke the new skills when the App's `targets` calls for them; eval gates fire on every run.
4. **M3 — Healing L1/L2**: enable detect + propose-fix; route to the Approvals Inbox.
5. **M4 — Reference workflow registration**: register `forge.reference.intent-to-infrastructure@1` in the workflow registry; mark it `kind=reference`; document.
6. **M5 — Demo target**: ship `make demo-intent-to-infrastructure`; verify the full opt-in chain runs end-to-end on the local stack.
7. **M6 — Friendly view default**: switch the Friendly view's "Nueva App" CTA to start the new workflow (after `alfred-console-redesign` ships).
8. **M7 — Promote**: as skills meet the promotion thresholds, advance from T1 to T2/T3 and enable in pilot tenants.

**Rollback**: feature flags off restores the previous orchestrator behaviour. The `App.targets` column stays in place (defaults remain valid).

## Open Questions

- Should the IaC apply runner be a separate service or reuse `deploy-orchestrator`? Strawman: a thin runner under `deploy-orchestrator` for now; promote to a service if scope grows.
- For L2 propose-fix, do we want to allow the operator to *edit* the proposed patch before approving? Recommendation: yes, render the diff in a Monaco editor inside the Approvals Inbox card; treat edits as an annotation that flows back into the eval feedback loop.
- For the design skill, the UI blueprint format: Figma export JSON vs Penpot vs a Forge-native format? Recommendation: Figma export JSON (industry standard) with a documented conversion path to Penpot for tenants that prefer open-source.
- Cross-App IaC modules (shared VPC, shared KMS): out of scope here; tracked separately.
