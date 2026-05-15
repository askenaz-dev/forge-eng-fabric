## Why

The SDLC orchestration in Forge today lights up the **plumbing** for every phase (Architecture, Design, QA, DevOps, SRE, FinOps, Security) and ships the `forge.reference.intent-to-deploy@1` reference workflow that walks an intent from spec to staging+prod deployment. What's missing is the actual **delivered capability** for five of those phases: most specs declare the *gates* and *skill names* but the skills are either unimplemented (`sdlc-design`, `sdlc-architecture`, `sdlc-qa`) or partially implemented and never wired into a real run (`healing-engine` for L1-L2, `sdlc-infrastructure`). The result is that an end-to-end run from an intent today stops at "PR opened with scaffolded code"; it does not produce an API contract, a data model, a threat model, a UI blueprint, generated tests, IaC, or auto-detected/auto-suggested healing patches. This change closes those five gaps and ships a new reference workflow — `forge.reference.intent-to-infrastructure@1` — that strings them together with opt-in steps so an App owner can declare exactly how deep the platform should drive the SDLC.

## What Changes

- **sdlc-design (new capabilities)**: implement `generate-ui-blueprint`, `generate-component-stubs`, `accessibility-audit` skills and wire them into the SDLC orchestrator. Output: a low-fidelity UI blueprint document (Figma-export-compatible JSON), React/Vue component stubs in the App's portal-bundle repo, and an Axe-driven accessibility audit report blocking progression when `criticality≥medium`.
- **sdlc-architecture (wire existing skills)**: implement and wire `generate-api-contract`, `propose-data-model`, `lightweight-threat-model` into the orchestrator. The current spec describes them; the change ships the skill code, the eval suites, the OpenSpec decision-log integration and the gate evaluator.
- **sdlc-qa (wire existing skills)**: implement and wire `generate-test-plan`, `generate-e2e-tests`, `triage-test-failures`. Add a *reactive CI triage* hook: when CI fails on the App's repo, the platform automatically invokes `triage-test-failures` against the failing run, posts a PR comment with the proposed fix and (optionally) opens a fix PR — this is the QA hand-off into the healing loop.
- **healing-engine L1–L2 (new capabilities)**: implement the `detect` and `propose-fix` pipelines explicitly at L1 (Notify) and L2 (Suggest), bridging from the existing `diagnosis-pipeline` and `healing-action-catalog` specs. Every detection event SHALL produce an L1 notification with HITL context; every L2 invocation SHALL surface a proposed fix (a code diff or a configuration change) and route it through the Approvals Inbox.
- **sdlc-infrastructure (new capability)**: introduce a brand new spec `sdlc-infrastructure` with `generate-terraform`, `generate-helm-values`, `validate-iac`, `apply-iac` skills. The capability fits between DevOps and SRE in the SDLC and ships Terraform modules for AWS/GCP/Azure (the three providers the platform already supports for runtimes), Helm value templates per criticality tier, a validator that runs `terraform plan` + `helm lint` + `conftest` against the App's policy bundle, and an applier that opens a PR for review before any `terraform apply` runs.
- **New reference workflow `forge.reference.intent-to-infrastructure@1`**: register a versioned workflow in `workflow-runtime` that strings together intent → architect → design → development → test → iac → deploy → observability. Each step is opt-in via a `targets:` map declared at the App level (e.g., `targets: { design: required, test: optional, iac: required, observability: opt-in }`). The workflow honours the targets, skips unselected steps and emits a structured plan reflecting the choices. The existing `forge.reference.intent-to-deploy@1` workflow remains as the smaller default; the new workflow becomes the canonical "go all the way" run.

## Capabilities

### New Capabilities

- `sdlc-infrastructure`: the IaC generation, validation and apply skills + gates, sitting between DevOps and SRE in the SDLC phase ordering.
- `intent-to-infrastructure-reference-flow`: the new reference workflow definition, its registration in the workflow registry, its target-opt-in semantics, the smoke test and the runbook.

### Modified Capabilities

- `sdlc-design`: ADD `generate-ui-blueprint`, `generate-component-stubs`, `accessibility-audit` skills with full scenarios + gates `ui_blueprint_present`, `component_stubs_committed`, `accessibility_audit_passed`.
- `sdlc-architecture`: MODIFY the existing skill enumeration to bind concrete implementations and add gate evaluator scenarios; ADD the OpenSpec decision-log integration scenarios.
- `sdlc-qa`: ADD reactive triage on CI failure with PR-comment + auto-fix-PR scenarios; MODIFY the existing `triage-test-failures` requirement with the reactive trigger.
- `healing-engine`: ADD L1 detection scenarios (notification with HITL context) and L2 propose-fix scenarios (Approvals Inbox routing). These complement the existing `Five-level healing model` requirement without changing it.
- `sdlc-orchestration`: ADD the `targets:` opt-in semantics for App-level workflow declaration; MODIFY phase-aware-capabilities to include `sdlc-infrastructure`.
- `application-entity`: ADD `targets` field on the App entity used by the reference workflow.

## Impact

- **New services / packages**: `services/sdlc-iac` (Go) hosting the IaC skill servers; `services/sdlc-design-skills` (Python/FastAPI) hosting the UI blueprint and a11y skills; `services/sdlc-architecture-skills` (Python/FastAPI) and `services/sdlc-qa-skills` (Python/FastAPI) for the wiring layer. Existing `healing-engine` service gains L1/L2 handlers.
- **New CloudEvents**: `sdlc.ui_blueprint.proposed.v1`, `sdlc.component_stubs.committed.v1`, `sdlc.accessibility_audit.completed.v1`, `sdlc.api_contract.proposed.v1`, `sdlc.data_model.proposed.v1`, `sdlc.threat_model.completed.v1`, `sdlc.test_plan.proposed.v1`, `sdlc.test_failure.triaged.v1`, `sdlc.iac.generated.v1`, `sdlc.iac.validated.v1`, `sdlc.iac.applied.v1`, `healing.detected.v1`, `healing.fix_proposed.v1`, plus a `workflow.intent_to_infrastructure.*` series.
- **Registry**: 11 new skill assets registered at T2/T3 with eval suites; the new reference workflow registered as an `asset_type=workflow` with `kind=reference`.
- **App schema**: add `targets` JSONB column on `application` with default `{architect: required, design: optional, development: required, qa: required, security: required, devops: required, sre: optional, finops: opt-in, iac: opt-in, observability: opt-in}`.
- **Workflow runtime**: extend the DSL with the `targets:` map; the runtime SHALL evaluate the targets at step entry and either run, skip, or pause for HITL based on the per-target setting.
- **HITL UI**: every L2 propose-fix surfaces in the existing Approvals Inbox with a diff renderer (existing approval card extended).
- **Dependency on prior changes**: `app-first-class-entity` (for `App.targets` and the App-scoped workflow runs) and `design-system-catalog` (so the design step can pick the App's Design System for the component stubs).
- **Smoke test**: extend the existing `make demo-intent-to-deploy` to a new `make demo-intent-to-infrastructure` target exercising the full opt-in chain end-to-end with a canned intent.
