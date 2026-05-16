## Why

The platform's north-star promise is that a **non-technical user expresses an intent in natural language and receives a deployed, AI-programmed solution end-to-end** — from requirements capture (Alfred), through architecture/design/QA/IaC produced by LLMs, into a real repository, real CI/CD, real Terraform/Helm artifacts applied to real infrastructure, ending in a live URL the user can access, with a single Forge bill covering LLM, cloud, and platform costs.

A direct gap audit (May 2026) found that **the entry point (Alfred) and the exit point (deploy-orchestrator's kubectl/gcloud machinery) are real, but everything in between is stubbed or disconnected**:

- The four SDLC skills do not call an LLM. `sdlc-architecture-skills` and `sdlc-qa-skills` ship real plumbing — they write files to disk and emit CloudEvents — but the content comes from deterministic templates (`_llm_call()` returns `{}`). `sdlc-design-skills` and `sdlc-iac` are deeper stubs: design returns a hardcoded Figma-template shape, IaC generation records path strings without writing files, and `apply-iac` fabricates a PR URL without invoking Git.
- `app-onboarding` creates the GitHub repo and applies branch protection, but **never commits or pushes the scaffolded code**. Repos end up empty.
- `deploy-orchestrator` bypasses the IaC chain entirely — it consumes a manifest from the request body, ignoring whatever IaC produced (or didn't produce).
- `model-gateway` now exists as a **stub Go service** (`services/model-gateway/`, scaffolded by the archived `ai-flow-authoring` change, task 6.1) with `POST /v1/resolve` backed by a `StubResolver` covering Claude/GPT models and per-workspace `allowed_models` whitelist enforcement. What's still missing is the production gateway layer: Go/Python SDKs that inject standard headers (`forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, `data_classification`); a `POST /v1/chat` endpoint with streaming; structured routing config by `cost_class`/`data_classification`; per-app cost attribution; model-version pinning. LiteLLM remains deployed via Helm with Cilium network policies blocking direct provider egress. Alfred in production (`services/alfred/alfred/llm.py:48`) bypasses both the new gateway AND header injection by calling LiteLLM directly. The LLM step executor in workflow-runtime (added by `ai-flow-authoring` task 4.5) correctly routes through `model-gateway`'s resolver, but its `HTTPLLMProvider` production path still goes to LiteLLM directly without the gateway-level mediation that F0c will add.
- The visual flow authoring surface **was delivered** by the archived `ai-flow-authoring` change (2026-05-16, 76/76 tasks): React Flow canvas, 5 trigger types, LLM node with `gateway:model/<id>@<channel>` references, custom node SDK v0 (workspace admin pastes a manifest URL via `/admin/custom-nodes/`). Side-effect: ai-flow-authoring also scaffolded `services/prompt-template-service/` (Go, `POST /v1/render` with `StubRenderer`) as a new sibling of the older `services/prompt-registry/` (Python, no production consumers). Two prompt services now coexist — F1b adopts the new one; consolidating/deprecating the old one is a V2 follow-up. Custom node publish API / marketplace / sandboxing / signing remains pending (Track B `connector-lifecycle`).
- HITL gates in demos are bypassed via `X-Forge-Demo-Auto-Approve: true`, masking the fact that production flows would block. The demo also forces every per-target inclusion (`architect:required`, `iac:opt-in`, etc.) to `required`, hiding the workflow's native conditional gating.

Beyond the technical gap above, two product-shaped gaps surfaced during the May 2026 design conversation:

- **User journey is unsolved.** A 20-40 minute end-to-end pipeline needs a UX that holds together when the user isn't watching. Cost authorization for non-technical users requires a model that doesn't force per-phase approvals. Post-deploy iteration ("make the button blue") needs a different code path than the initial intent. Multi-app management has no surface today.
- **Business model is unsolved.** Today there is no billing infrastructure: no usage metering, no tier model, no payment integration, no cloud-account provisioning, no LLM cost markup. Shipping the technical pipeline without these means we have a demo, not a product.

This umbrella change **does not implement any of the above**. It captures the gap map, declares the cohesive program of work needed to close it, names the sub-changes that will carry each piece, sequences them by dependency, and records the seventeen product decisions taken on 2026-05-16 that establish the operating model.

## Scope of this change

This is an **umbrella / roadmap change**. It produces no code and no spec deltas. It establishes:

- A shared narrative and gap inventory the team and Alfred can reference.
- The set of follow-on sub-changes, with names and one-paragraph scopes.
- The dependency ordering between sub-changes.
- The seventeen product decisions that govern sub-change design.
- The acceptance criteria for declaring the umbrella complete (i.e., when intent-to-infrastructure works end-to-end against the reference workflow with no stubs, with a usable UX, and with billing flowing through).

Each front below will be split into its own OpenSpec change with its own `proposal.md`, `design.md`, `tasks.md`, and `specs/` deltas. **This umbrella is the index.**

## Acceptance criteria for the umbrella

The umbrella is considered closed when all of the following are true on `main`:

**Technical pipeline (intent → URL):**

1. A non-technical user submits an intent via Alfred (friendly view) within a workspace that has an active app context (or selects no-context to create a new app).
2. Alfred completes intent capture via conversational follow-ups, dispatches `forge.reference.intent-to-infrastructure@1` in agent-mode, and streams progress.
3. Every SDLC phase (architecture, design, QA, IaC) produces artifacts generated by a real LLM, routed through `model-gateway`, charged to the user's workspace budget, with output validated against a published JSON schema and persisted in the asset registry.
4. The generated source code is committed and pushed to a GitHub repository in the tenant's GitHub Org (per Decision 5), with CI configured and passing on first push, and `gitops/` populated with an ArgoCD/Flux Application CR.
5. The generated Terraform/Helm artifacts exist as real files in the app repo's `infra/` folder. For staging, the server-side `iac-apply-runner` executes `terraform apply` against the per-tenant **GCS** state backend (co-located with the tenant's cloud project). For production, Atlantis applies after a human approves the PR. Both paths produce real cloud resources in GCP.
6. `deploy-orchestrator` reads IaC outputs from the state backend, renders the deploy manifest, commits it to the app repo's `gitops/` folder, and exits. The GitOps reconciler (ArgoCD or Flux per F3) picks up the change and rolls out the workload.
7. The generated app has authentication enabled by default (per Decision 9) with auto-generated temporary admin credentials delivered to the intent initiator via email.
8. The user receives a live URL plus hero card (URL + repo link + admin creds + console link + LLM-generated summary + traceability expandable). `traceability-graph` links intent → spec → repo → IaC state → deployment → URL.

**User journey (the platform feels like a product):**

9. The status panel renders phase-level progress via SSE; chat surface emerges only at decision points (HITL gate, cost cap pause). Email notifications fire when user is away.
10. Per-intent Autopilot mode is available; when enabled it bypasses HITL gates, soft-cap pauses, and intermediate notifications without bypassing the hard cap or compliance gates.
11. Post-deploy iteration works: with an app selected in the AppPicker, the user says "make the button blue" and Alfred routes via F6 (codebase-aware) to a focused PR; CI + GitOps reconcile the change.
12. Multi-app dashboard lists apps in the current workspace with cost, status, and supports decommission (soft, terraform-destroy only), re-deploy, and per-app cost view.

**Business model (the platform is monetizable):**

13. All LLM calls are observable in `ai-observability` with per-workspace, per-app cost attribution; no service calls a model provider directly.
14. F7 produces an accurate monthly invoice for the test tenant, combining reseller-LLM markup + cloud usage rollup + email allowance + platform subscription.
15. F8 onboards a new test tenant end-to-end: tenant installs Forge GitHub App on their GitHub Org, selects a GCP region, F8 provisions a GCP Project with baseline IAM/networking/Secret Manager, billing rollup begins flowing to F7.

## Fronts (sub-changes to be opened)

Fifteen fronts total. Each becomes a separate OpenSpec change. Provisional naming.

### Quick-fix — `alfred-litellm-header-injection` (sibling, no umbrella dependency)

**Scope.** Fix two production bugs surfaced by the gap audit:

- **G1.** `services/alfred/alfred/llm.py:48` calls LiteLLM with plain auth — does not inject `forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, `data_classification` headers despite the stub doing so correctly.
- **G2.** `services/alfred/alfred/tools.py` `ToolRouter` routes `prompt:*` tool calls to `prompt-registry`'s `/v1/invoke` endpoint that does not exist in the registry's `app.py`. **Note (2026-05-16):** since the archived `ai-flow-authoring` shipped `prompt-template-service` (Go) with `POST /v1/render`, the recommended fix is to **migrate Alfred's `ToolRouter` to call `prompt-template-service/v1/render`** (architecturally consistent with the workflow-runtime LLM executor) rather than implementing `/v1/invoke` in the legacy `prompt-registry`.

**Why separate.** Independent of F0; does not need SDKs to land. 1-2 days. Ship ASAP regardless of F0 sequencing.

**Capabilities touched.** None formally (bug fixes); document in change log.

### F0a — `model-gateway-spec-amendment`

**Scope.** Specification-only change. The `services/model-gateway/` Go service now exists (scaffolded by archived `ai-flow-authoring` with stub resolver) but its spec lacks concrete requirements for: Go and Python SDK contracts wrapping it; a production `POST /v1/chat` endpoint with streaming; structured routing config schema (selection by `cost_class` and `data_classification`); per-app / per-asset cost attribution; model-version pinning; rate-limit separated from budget. Amend `openspec/specs/model-gateway/spec.md` to add these and document the existing stub `/v1/resolve` as the baseline contract that F0b/F0c must preserve.

**Blocks.** F0b, F0c.

**Capabilities touched.** Modifies `model-gateway` (spec only, no code).

### F0b — `model-gateway-sdks`

**Scope.** Two pieces:
1. Add a `POST /v1/chat` endpoint to `services/model-gateway/` (today only `/v1/resolve` exists). The endpoint accepts chat completions, applies routing rules (F0c work but contract here), and forwards to LiteLLM upstream. Preserves the existing `/v1/resolve` contract used by workflow-runtime's `HTTPModelResolver`.
2. Implement Go and Python SDK clients that wrap `model-gateway` (not LiteLLM directly) and inject the standard headers required for tenant-scoped routing, budget, and observability. Migrate every direct-LiteLLM callsite: Alfred today (`llm.py:48`), F1a, F1b, and the workflow-runtime LLM executor's `HTTPLLMProvider` (currently bypasses the gateway).

Includes header-injection correctness tests and end-to-end test against the `ai-email-triage` reference flow that ai-flow-authoring shipped (the existing flow becomes a regression smoke for F0b).

**Why separate from F0c.** Header injection is the highest-value, lowest-risk piece of F0 — it unblocks correct cost attribution today even before routing/observability lands. Coordinate sequencing with the Quick-fix front: either Quick-fix lands first as a stop-gap, or F0b absorbs it.

**Depends on.** F0a.

**Blocks.** F1a, F1b, F7 (billing needs accurate cost ingestion). Must preserve contract with archived `ai-flow-authoring`'s LLM executor (regression smoke against `ai-email-triage` reference flow).

**Capabilities touched.** Modifies `model-gateway`. Adds bootstrap requirements to `alfred-control-plane`.

### F0c — `model-gateway-routing-and-cost`

**Scope.** Implement the routing layer (LiteLLM router config driven by `data_classification` and `cost_class`), per-app cost attribution end-to-end (LiteLLM cost telemetry → FinOps ingestion → `ai-observability` surface → F7 invoice rollup), Langfuse hooks for prompt/response/cost/latency per request with redaction, rate-limit enforcement separate from budget, model-version pinning enforcement, and BYOK pass-through (when tenant supplies their own key, no markup, attribution still works).

**Depends on.** F0a, F0b.

**Blocks.** F7 (billing engine ingestion contract).

**Capabilities touched.** Modifies `model-gateway`, `finops`, `ai-observability`.

### F1a — `alfred-via-model-gateway`

**Scope.** Cut Alfred's direct `LiteLLMClient` calls and route every reasoning loop, dialogue follow-up, and planner call through the F0b SDK. Includes budget probe migration and the agent-mode planner. If Quick-fix already landed header injection, F1a still applies — the SDK replaces the manual header bookkeeping with the official client.

**Depends on.** F0b.

**Capabilities touched.** Modifies `alfred-control-plane`.

### F1b — `sdlc-skills-llm-realization`

**Scope.** Replace the template-based generation block in each SDLC skill (architecture, design, QA, IaC) with a real LLM call routed through F0b's SDK. Two distinct work profiles:

- **Architecture & QA skills** — plumbing already exists (file persistence, CloudEvent emission, registry surfacing). Work is a localized find-and-replace of the `_llm_call() returns {}` block plus prompt authoring and output validation. Smaller than initially estimated.
- **Design & IaC skills** — deeper stubs. Design returns a hardcoded Figma-template shape; IaC records path strings without writing files. Need both LLM integration AND file-emission plumbing.

For each skill: author a prompt template in **`prompt-template-service`** (the Go service shipped by archived `ai-flow-authoring`, already wired into workflow-runtime's LLM executor). The older `prompt-registry` Python service is becoming legacy and has no production consumers — adopting prompt-template-service is the architecturally consistent choice; consolidating/deprecating prompt-registry is a follow-up (see Out of scope). Define a JSON Schema for output validation alongside each template. prompt-template-service ships a `StubRenderer` today; F1b loads real production templates for each SDLC skill. Validate every LLM response client-side in the skill until prompt-template-service grows a `POST /v1/templates/{id}/validate-output` endpoint (small follow-up). prompt-template-service may need tenant-scoped template overrides (so a tenant can customize their architecture prompt) — F1b adds this requirement.

Retry/fallback policy for invalid JSON or budget exhaustion is defined per-skill. Per-phase artifacts continue surfacing to `traceability-graph` and the asset registry (already wired for arch/QA; add for design/IaC).

**Depends on.** F0b.

**Capabilities touched.** Modifies `sdlc-architecture`, `sdlc-design`, `sdlc-qa`, `sdlc-orchestrator`. Modifies `prompt-template-service`.

### F2 — `scaffolder-to-repo-pipeline`

**Scope.** Insert a `github.commit_and_push` stage in `app-onboarding` between `scaffold.render` and `github.codeowners`. Within the tenant's GitHub Org (per Decision 5), provision the workspace's GitHub Team if absent, then create the app's repo within that team. Clone the repo, copy scaffolded files (app source with **auth layer enabled by default per Decision 9** + `.github/workflows/` + `gitops/` with ArgoCD/Flux Application CR template + `infra/` placeholder), commit, push, configure OIDC federation to GCP, set CODEOWNERS to the intent initiator, verify first CI green.

**Tenant onboarding** (separate from per-app scaffold) handles:
- "Install GitHub App on your org" step (per Decision 5).
- "Choose your GCP region" step (per Decision 14, curated list).
- For default tenants: trigger F8 to provision a GCP Project.
- For BYOC tenants: capture GCP Project ID and IAM trust setup.
- Persist installation ID, region, and project ID per tenant.

**Why parallelizable.** No LLM dependency. Can ship in parallel with F0/F1b.

**Depends on.** F8 (for default-tenant cloud provisioning during onboarding). The reconciler tool (ArgoCD vs Flux) is determined in F3 and affects the Application CR template format — see Risks.

**Capabilities touched.** Modifies `app-onboarding`, `repo-template-catalog`, `ci-pipeline-baseline`. Adds requirements to `github-app-provisioning` for GitHub App installation flow and OIDC federation setup. Adds default auth-scaffolding to every language/runtime template.

### F3 — `iac-realization`

**Scope.** Replace `sdlc-iac`'s stub `GenerateTerraform`/`GenerateHelmValues` with real file generation (LLM-driven, validated against JSON Schema, rendered to actual `.tf` / `values.yaml` files). Replace the fake PR URL in `ApplyIaC` with a real Git operation: commit IaC files to the app's `infra/` folder and open a real PR. Build both execution paths per Decision 3:

- **Sandbox/staging:** new `iac-apply-runner` service runs `terraform apply` server-side with per-tenant credentials against the **GCS state backend** (per Decision 3 revised — Decision-3-Update below) in the tenant's GCP Project.
- **Production:** PR-driven via **Atlantis** (self-hosted, multi-tenant config), gated by GitHub approval aligned with `approve-prod-deploy` HITL.

Cluster bootstrap (part of each runtime's IaC) installs the GitOps reconciler — **the choice between ArgoCD and Flux is made in this front**. The reconciler watches the `gitops/` folder of every app repo onboarded to the cluster, with per-tenant RBAC isolation. Cluster runtimes are GKE or Cloud Run (GCP-only per Decision 13).

**Depends on.** F1b (for real IaC content) and F2 (for the repo to commit into).

**Capabilities touched.** Modifies `sdlc-iac`, `iac-drift-detection`. Adds new capabilities `iac-apply-runner` (server-side path) and `iac-pr-driven-apply` (Atlantis wiring). Adds requirements to `runtime-connectors` for GKE/CloudRun + reconciler installation + per-tenant RBAC.

### F4 — `deploy-iac-coupling`

**Scope.** Restructure `deploy-orchestrator` to depend on IaC apply output (Terraform state outputs: runtime endpoint, namespace, secret references, reconciler endpoint) rather than the request body. The reference workflow's `deploy-staging` step's `depends_on` changes from `ci-pipeline-setup` to `apply-iac`.

Per Decision 4, deploy-orchestrator becomes a **manifest committer** by default: renders the deploy manifest using IaC state outputs, commits to the app repo's `gitops/` folder, opens/updates a PR, exits. The GitOps reconciler (installed per F3) does the apply. The orchestrator also exposes a **push escape hatch** (`kubectl apply` / `gcloud run deploy`) gated by a feature flag for bootstrap, hotfix, and debugging; escape-hatch use is audited and tied to a documented approval reason.

Update `runtime-registry` to ingest runtime metadata from Terraform outputs rather than fake provisioner records. Read state from the GCS backend (Decision 3 revised).

**Depends on.** F3.

**Capabilities touched.** Modifies `deploy-orchestrator`, `runtime-registry`, `runtime-connectors`. Adds new capability `gitops-deploy-coupling`.

### F5 — `alfred-agent-mode-intent-to-infra`

**Scope.** Wire Alfred's agent-mode planner to dispatch `forge.reference.intent-to-infrastructure@1` (instead of the lighter `intent-to-deploy@1` fallback) when no app context is set. When app context IS set, dispatch the iteration flow (handled by F6). Adapt the friendly-view UI to consume the F10 status panel + chat surfaces. Surface the final hero card prominently. Implement the "resolve hasta deploy" retry policy with stop-at-hard-cap. Remove `X-Forge-Demo-Auto-Approve` from demo paths.

**Depends on.** F1a, F4, F9 (Autopilot signal), F10 (UX surfaces), F6 (iteration dispatch).

**Capabilities touched.** Modifies `alfred-control-plane`, `sdlc-orchestrator`. Modifies `workflow-runtime` (streaming contract).

### F6 — `codebase-aware-alfred-iteration` (NEW)

**Scope.** Equip Alfred to handle post-deploy iteration on an existing app (Decision 10 hybrid model). Three capabilities:

- **Codebase awareness.** Alfred can read the contents of an app's repo (clone or sparse-fetch via the GitHub App installation), build a semantic index (file tree + key files + symbol map), and ground LLM prompts in that index.
- **Change classification.** Given an iteration intent ("make the button blue", "add login with Google", "switch DB to Postgres") and the codebase index, classify the change as cosmetic, new-feature, or architectural.
- **Routing.** Cosmetic → Alfred generates a focused PR with the change (no SDLC pipeline). New-feature → re-runs design + dev phases on the existing arch. Architectural → full pipeline re-run, version-bumped.

**Depends on.** F1a (Alfred → SDK), F5 (UI surface).

**Capabilities touched.** Adds new capability `codebase-aware-iteration`. Modifies `alfred-control-plane`. May extend `sdlc-orchestrator` to support partial-pipeline dispatch.

### F7 — `billing-and-metering` (NEW)

**Scope.** Build the platform's billing and metering layer per Decision 12 (LLM reseller) + Decision 13 (cloud reseller) + Decision 15 (email reseller):

- **Usage metering ingestion.** Consume LLM cost events from F0c, GCP billing/Cost Allocation tags from F8-provisioned projects (default tenants), SendGrid usage from email integration.
- **Pricing engine.** Apply markups, included allowances, tier rules. MVP tier "Basic" with predefined numbers (Decision 7).
- **BYOK/BYOC pass-through.** When tenant uses BYOK LLM or BYOC cloud, no markup applied; cost telemetry still flows but invoice line shows pass-through.
- **Payment integration.** Stripe Connect (or equivalent) for tenant payment methods, prepay balance, autocharge, dispute handling, fraud detection.
- **Customer-facing dashboard.** Tenants see per-workspace, per-app cost breakdown; set hard caps; manage payment methods.
- **Internal admin.** Operator view of margin, abusers, refund flow.
- **Hard cap enforcement.** Coordinates with model-gateway and deploy-orchestrator to halt at workspace limit.

**Estimated effort.** 2-3 months full-time team. Stripe Billing reduces the build by half.

**Depends on.** F0c (cost ingestion contract).

**Capabilities touched.** Adds new capability `billing-and-metering`. Modifies `finops`, `ai-observability`. New service `services/billing/`.

### F8 — `tenant-cloud-provisioning` (NEW)

**Scope.** Automate per-tenant GCP project provisioning for default (non-BYOC) tenants per Decision 13:

- **GCP Organization + Folders setup.** One-time bootstrap of our GCP Organization with a Folder per tenant.
- **Per-tenant Project creation** at onboarding: new Project under the tenant's Folder, baseline VPC, IAM roles (Forge control plane trust), Secret Manager bootstrap, Cost Allocation labels for F7 rollup.
- **Region application** per tenant's onboarding choice (Decision 14).
- **BYOC path** (no F8 work): tenant pastes their GCP Project ID, installs our IAM policy, F8 stores the trust and skips provisioning. F8's onboarding code handles both paths.
- **Cleanup at churn.** When tenant deactivates, optional Project deletion (with grace period) or transfer-to-tenant.

**Estimated effort.** 2-3 months. AWS support deferred to V2 per Decision 13.

**Depends on.** Nothing in this umbrella (depends on us having a GCP Organization set up out-of-band).

**Capabilities touched.** Adds new capability `tenant-cloud-provisioning`. New service `services/cloud-provisioner/`.

### F9 — `autopilot-mode` (NEW)

**Scope.** Per-intent opt-in Autopilot mode per Decision 8:

- **Per-intent checkbox** at intent kickoff: "autopilot — don't interrupt me".
- **Bypass logic in the workflow:** HITL gates auto-approved, cost soft-cap pause skipped (auto-continue to hard cap), intermediate notifications suppressed (only final notification fires).
- **Non-bypassable:** workspace hard cap, catastrophic failures (terraform plan errors, region capacity), policy-marked compliance gates.
- **Reconfirmation cadence.** Periodically (every N intents in autopilot, or every M days) reset the autopilot opt-in and re-ask.
- **Audit.** Every autopilot-driven decision logged: which gate was bypassed, which user enabled the mode, the workspace.

**Depends on.** F5 (intent dispatch), F10 (UX render).

**Capabilities touched.** Adds new capability `autopilot-mode`. Modifies `alfred-control-plane`, `policies-and-approvals`.

### F10 — `user-journey-ux` (NEW)

**Scope.** Build the Cluster A + Cluster B + Cluster C surfaces of the user journey:

- **Status panel** (live SSE stream from workflow CloudEvents) with phase progression, per-phase artifact previews collapsible, cancel button.
- **Chat surface** integrated with Alfred; surfaces only for HITL gates, cost cap pauses, intent classification ambiguities, Alfred-driven failure diagnosis.
- **Email notifications** as fallback when user closes browser: at decision points, at completion, at cancellation, at failure. Uses Decision 15 email infrastructure.
- **Hero card at completion** (Decision 9): URL + repo link + admin creds (revealed on click) + console link + LLM summary + traceability expandable.
- **AppPicker.tsx** in portal — sibling of `WorkspacePicker.tsx`. Setting an app scopes all Alfred intents to that app (per Decision 10).
- **Multi-app dashboard** at `/apps` — list view per workspace with status, cost, last deploy. Per-app actions: re-deploy, decommission (soft per Decision 11), view costs.
- **Cancel post-IaC dialog**: warn-then-destroy per Decision 6 (clear copy, terraform-destroy on confirm).

**Depends on.** F4 (manifests committed), F5 (dispatch signals), F6 (iteration UI), F9 (autopilot checkbox), F7 (cost display).

**Capabilities touched.** Modifies `portal` surfaces. New components in `portal/src/components/`. New routes in `portal/src/app/`.

### Track B extension — `connector-lifecycle` (follow-up to archived `ai-flow-authoring`)

**Scope.** The archived `ai-flow-authoring` change (2026-05-16) shipped the custom node SDK at v0: `docs/sdk/custom-nodes.md` contract, example `uppercase-text` node, `/admin/custom-nodes/` form for workspace admins to paste manifest URLs, `stubActivity` in workflow-runtime for `StepCustom`. Ingestion pipeline (publish API, marketplace, sandboxing, signing) was explicitly out of scope. Since end-user extensibility ("conectores que se desarrollen") is a stated product requirement, this front closes the gap: connector publish API, per-workspace authorization, sandbox runtime + JWT signer + endpoint dispatcher (workflow-runtime stub returns `step_type_not_yet_implemented` until this lands), marketplace listing, signing / supply-chain attestation reuse.

**Why separate.** Not on the intent-to-infra critical path. Belongs to Product B (visual flow authoring) rather than Product A (intent-to-infra), but shares substrate (registry, policy, billing).

**Depends on.** Nothing in this umbrella (`ai-flow-authoring` already shipped and laid the foundation).

**Capabilities touched.** Adds new capability `custom-connector-lifecycle`. Modifies `mcp-and-skills`, `tenant-workflow-marketplace`, `supply-chain-attestations`.

## Dependency graph

```
   Quick-fix (independent, 1-2 days):
       alfred-litellm-header-injection
       (fixes G1 + G2; sibling to F0b)

   ┌─────────────────────── CRITICAL PATH: intent → URL ────────────────────┐
   │                                                                        │
   │   F0a model-gateway-spec-amendment                                     │
   │            │                                                           │
   │            ▼                                                           │
   │   F0b model-gateway-sdks ─── (preserve contract w/ shipped ai-flow)   │
   │            │                                                           │
   │            ▼                                                           │
   │   F0c model-gateway-routing-and-cost ────► F7 billing-and-metering     │
   │            │                                                           │
   │   ┌────────┴────────┐                                                  │
   │   ▼                 ▼                                                  │
   │ F1a alfred     F1b sdlc-skills                                         │
   │       │             │                                                  │
   │       │             ▼                                                  │
   │       │       F3 iac-realization ◀── F2 scaffolder→repo ──► F8 cloud  │
   │       │             │                       │                          │
   │       │             ▼                       │                          │
   │       │       F4 deploy↔iac (GitOps)        │                          │
   │       │             │                       │                          │
   │       └──────┬──────┘                       │                          │
   │              ▼                              │                          │
   │   F5 alfred→intent-to-infra ◀── F6 codebase-aware ◀── F9 autopilot     │
   │              │                                          │              │
   │              └──────────────────┬───────────────────────┘              │
   │                                 ▼                                      │
   │                       F10 user-journey-ux                              │
   │                                 │                                      │
   │                                 ▼                                      │
   │                          UMBRELLA CLOSES                               │
   │                                                                        │
   └────────────────────────────────────────────────────────────────────────┘

   Track B (independent):
     ai-flow-authoring (archived 2026-05-16; F0b preserves its contract)
                    │
                    ▼
     connector-lifecycle (post-ai-flow-authoring)
```

## Product decisions

Seventeen decisions closed on 2026-05-16, grouped in three buckets. All sub-changes can be designed against these. One residual sub-choice — ArgoCD vs Flux as the GitOps reconciler — is deferred to F3 implementation.

### Bucket A — Technical architecture

#### Decision 1 — Repo topology ✅ DECIDED: Mono-repo, 1 app = 1 repo (strict)

Generated app code, IaC, and GitOps manifests all live in a single repository per app. Strict cardinality: an app is exactly one repo at MVP; multi-repo apps (microservices) are out of scope for this umbrella.

Layout per repo:
- `app/` — application source
- `infra/` — Terraform / Helm sources
- `gitops/` — Kubernetes manifests + ArgoCD/Flux Application CR
- `.github/workflows/` — CI pipelines

CODEOWNERS ships with a single owner (the intent initiator) at MVP; per-path ownership deferred.

#### Decision 2 — CI/CD substrate ✅ DECIDED: GitHub Actions

`.github/workflows/` files in every scaffold. First push triggers CI. OIDC federation to GCP per-repo at onboarding. Hard dependency on GitHub accepted (consistent with Decision 5).

#### Decision 3 — `terraform apply` execution model ✅ DECIDED: Hybrid (Atlantis + server-side runner) + GCS state

- **Sandbox/staging:** new `iac-apply-runner` runs `terraform apply` server-side with per-tenant credentials.
- **Production:** PR-driven via **Atlantis** (self-hosted), gated by GitHub approval.
- **State backend:** **GCS with native object locking, per-tenant isolated** (one bucket in the tenant's GCP Project — co-located with cloud resources). Revised from the original S3+DynamoDB choice once Decision 13 set GCP as default cloud.

#### Decision 4 — App-layer deploy model ✅ DECIDED: Hybrid GitOps + push escape

- **Default:** GitOps. A reconciler (ArgoCD or Flux — choice in F3) per runtime watches `gitops/` and reconciles continuously. Cluster state = repo state.
- **Escape hatch:** `deploy-orchestrator` retains push path for bootstrap, hotfix, debugging. Feature-flagged and audited.

#### Decision 5 — Entity binding to GitHub ✅ DECIDED: tenant→org, workspace→team, app→repo

- **Tenant ↔ GitHub Org** — tenant-owned. Tenant installs our GitHub App on their existing org.
- **Workspace ↔ GitHub Team** within the tenant's org.
- **App ↔ 1 Repo** within the workspace's team.

Acknowledged lock-in: workspace identity anchored to GitHub Team means tenant cannot host workspaces on GitLab/Bitbucket. Consistent with Decision 2.

### Bucket B — User journey

#### Decision 6 — Wait UX during long-running intents ✅ DECIDED: status panel + chat hybrid + email fallback (Pattern ε)

A 20-40 minute end-to-end needs UX that's scannable when ignored and clear when needed.

- **Status panel** (live SSE) shows phase progression with collapsible per-phase previews.
- **Chat surface** stays silent until a decision point (HITL gate, cost cap, intent ambiguity).
- **Email fallback** when user closes browser; one notification per pending decision or completion.
- **Sessions are server-side**; browser is a view. Closing tab does not stop execution.
- **Cancel** always available. Post-IaC: warn-then-destroy ("we'll tear down provisioned infra, confirm?").
- **Failure handling**: Alfred self-heals and retries aggressively ("resolve hasta deploy"). Hard cap on workspace budget is the stop. Stop = honest failure, user pays what was spent, no partial refund.

#### Decision 7 — Cost authorization ✅ DECIDED: tier-based with soft+hard caps + reference-table estimation

- **MVP tier**: single "Basic" tier with predefined numbers. Tenants self-serve adjust their infra repo to optimize costs.
- **Per intent**: Alfred estimates ex-ante from a reference table of similar deployed apps ("estimated $12, similar to app X"). MVP: hand-curated table. V2: learned model.
- **During run**: auto-continues until soft cap (60% of intent max). At soft cap, pauses and asks "going higher than expected, continue up to ${max}?"
- **Hard cap**: remaining workspace budget. Never exceeded.
- **On budget exhaustion**: stop, surface failure. User pays for what was spent.

#### Decision 8 — Autopilot mode ✅ DECIDED: per-intent opt-in (Claude Code "Bypass permissions" pattern)

- **Per-intent checkbox** at kickoff: "autopilot — don't interrupt me".
- **Bypasses**: HITL gates, soft-cap pause, intermediate notifications.
- **Does NOT bypass**: hard cap, catastrophic failures, policy compliance gates.
- **Reconfirmation**: periodic re-ask (every N intents or M days).
- **Audit**: every autopilot decision logged.

#### Decision 9 — Completion handoff ✅ DECIDED: hero card + email mirror + auth-by-default

Hero card at completion shows: URL, repo link, admin credentials (revealed on click), console link, LLM-generated summary, traceability expandable. Email mirrors the same content. Single notification at completion.

**Auth-by-default**: every generated app has authentication enabled unless the intent explicitly declares "public". Initial admin credentials are auto-generated (cryptographically random), stored in the tenant's GCP Secret Manager, delivered via Decision-15 email to the intent initiator.

#### Decision 10 — Iteration model ✅ DECIDED: hybrid + app context obligatory

- **App context**: portal has an AppPicker (sibling of WorkspacePicker). User selects an app to scope Alfred. All intents while scoped apply to that app. Without context, Alfred asks: "is this for a new app or one of [app1, app2, app3]?"
- **Hybrid routing** (per F6):
  - Cosmetic change → focused PR, CI + GitOps reconcile.
  - New feature → re-runs design + dev on existing arch.
  - Architectural change → full pipeline re-run.

#### Decision 11 — Multi-app management ✅ DECIDED: MVP scope + soft decommission

- **MVP operations**: list, decommission, view costs, re-deploy.
- **Deferred to V2**: pause/scale-to-zero, version history, clone, transfer ownership.
- **Decommission semantics**: soft-only. `terraform destroy` runs, repo stays (tenant-owned). User can re-deploy manually later at $0 cost.

### Bucket D — Business model

#### Decision 12 — LLM credentials ✅ DECIDED: Reseller default + BYOK enterprise

- **Default (Reseller)**: Forge holds provider accounts at scale. Tenant LLM usage routes through our accounts. Single Forge bill with markup covering cost + margin + abstraction value.
- **Enterprise opt-in (BYOK)**: tenant provides own provider keys. Forge passes through, no markup. Tenant receives provider bills directly.
- **Cost telemetry** flows in both modes.

#### Decision 13 — Cloud credentials ✅ DECIDED: GCP-provisioned default + BYOC GCP power-user

- **Default (Forge-provisioned)**: F8 creates a GCP Project per tenant under our Organization. Tenant is technically project owner; we operate billing.
- **Power-user opt-in (BYOC)**: tenant brings own GCP Organization + Project. Self-service (paste Project ID, install our IAM policy). Skips F8 entirely.
- **Cloud scope**: GCP-only at MVP. AWS = V2 best-effort if bandwidth allows. GCP-native, no abstraction layer for MVP.

#### Decision 14 — Multi-region ✅ DECIDED: tenant selects at onboarding

Onboarding presents curated GCP region list (us-central, europe-west, asia-northeast, etc.). All apps of a tenant deploy in their chosen region. Change after onboarding requires manual migration (out of scope).

#### Decision 15 — Email transactional ✅ DECIDED: SendGrid reseller + BYOK enterprise

- **Default**: SendGrid with Subusers API per tenant. Markup in bill.
- **Enterprise opt-in**: BYOK with tenant's own SendGrid / Mailgun / Postmark.
- Used for: credential delivery, completion notifications, cancel confirmations.

#### Decision 16 — App-needed secrets ✅ DECIDED: BYOK tenant in GCP Secret Manager

Secrets the generated app needs (Stripe, third-party APIs, OAuth provider secrets): tenant brings keys at intent capture time, stored in their GCP Secret Manager. Forge never sees values. Auto-generated auth creds (Decision 9) also live here.

#### Decision 17 — Product A ↔ Product B separation ✅ DECIDED: separate tracks, no composability at MVP

- **Product A** (intent → app): this umbrella's scope.
- **Product B** (visual AI workflow editor): `ai-flow-authoring` change.
- **No cross-contamination at MVP**: Alfred does not classify intent as app vs workflow; user enters via separate surfaces. Apps do not depend on workflows.
- Workflows in Product B can be deployed as managed HTTP endpoints (Flowise-style "Deploy as API") — this is a follow-up change to the archived `ai-flow-authoring`, NOT in this umbrella. ai-flow-authoring shipped `webhook-in` triggers (per-trigger HTTP entrypoint) but not the full "managed API endpoint with auth/rate-limit/monitoring" pattern. A separate sibling change to ai-flow-authoring would close this gap.

## Out of scope

Explicitly **not** in this umbrella:

**V1 deferrals:**
- Migrating away from LiteLLM (F0 wraps it).
- Promoting the lighter `intent-to-deploy@1` workflow (stays as fallback).
- Symptom-triggered autonomous sessions (healing flows are separate).
- Re-architecting workflow-runtime.
- Replacing Postgres anywhere.

**V2 candidates (will become fronts if validated):**
- AWS support (`F11-aws-cloud-expansion`). Decision 13 says GCP-only MVP.
- Apps using workflows as dependencies (`F12-app-workflow-composability`). Decision 17 V2.
- Alfred classifying intent app vs workflow (`F13-intent-classification`). Decision 17 V2.
- Multi-tier pricing (Decision 7 says single Basic tier at MVP).
- Per-app versioning, clone, transfer ownership (Decision 11).
- App pause / scale-to-zero (Decision 11 — only hard decommission).
- Per-path CODEOWNERS for mono-repo (Decision 1).
- Hybrid Forge-provisioned + BYOC at per-app granularity (Decision 13 — only per-tenant).
- Marketing/bulk email infrastructure (Decision 15 — only transactional).
- Multi-repo apps (microservices in separate repos) (Decision 1).
- Consolidating `prompt-registry` (Python, legacy, no production consumers) and `prompt-template-service` (Go, shipped by archived ai-flow-authoring, used by workflow-runtime). Recommend deprecating prompt-registry and migrating any latent references to prompt-template-service as a small follow-up change after F1b adopts the new service.

## Risks

- **Latency cliff.** Real LLMs in 4+ phases turn a "seconds" demo into a 20-40 min run. F10's UX work is mandatory, not cosmetic.
- **Non-determinism on LLM output.** Without JSON Schema validation (F1b explicit requirement), every fix to one phase risks breaking the next. The prompt-registry contract is load-bearing.
- **Credentials blast radius.** Even with BYOC option, default tenants have credentials concentrated in F8-provisioned Projects under our Organization. Security review on IAM trust setup is critical.
- **Deferred reconciler tool choice (ArgoCD vs Flux).** Resolved in F3. If F2 ships its scaffold template before F3 commits, the Application CR template may need migration. Mitigation: F2 scaffolds a CR-agnostic placeholder filled in at deploy time.
- **Track B independence is brittle.** `connector-lifecycle` shares the registry, policy, and billing substrate with everything in Track A. Sequencing review needed at each F0 milestone.
- **Billing infrastructure scope (F7).** 2-3 months full-time team. Stripe Billing can reduce by half but customer dashboard + usage rollup + dispute handling are real work. Mitigation: ship MVP with manual invoicing for first 5-10 design partners while F7 builds.
- **Cloud provisioning ops debt (F8).** GCP Organization + Folders + per-tenant Projects automation introduces new ops burden. Mitigation: BYOC tenants bypass F8 entirely; can ship to power-users while F8 stabilizes.
- **GCP-only MVP forecloses some enterprise.** Not all enterprises can use GCP (regulatory, existing AWS commitments). Mitigation: BYOC GCP serves most enterprise; AWS expansion (V2 front F11) is the planned fast-follow.
- **Iteration accuracy depends on codebase awareness.** If F6 ships subpar, Decision 10 hybrid degrades to "always re-run", which negates the iteration value. Mitigation: ship F6 alongside F5; both must be acceptable for umbrella close.
- **Autopilot mode liability surface (F9).** Bypassing HITL means a bypassed security-review could allow a vulnerable deploy. Mitigation: audit trail per autopilot decision + periodic reconfirmation + policy-marked gates non-bypassable.
- **Auth-by-default scaffold complexity.** Every language/runtime template (Decision 9) must include a working auth layer — meaningful template engineering. Mitigation: start with 2-3 stacks (Node/Express, Python/FastAPI, Go/Gin), expand as data warrants.

## Cross-change coordination

Some sub-changes share substrate with each other or with changes external to this umbrella (some archived, some still in flight):

### F0b ↔ archived `ai-flow-authoring` (HISTORICAL — RESOLVED)

This was a real risk before ai-flow-authoring shipped. **Outcome (2026-05-16):** ai-flow-authoring shipped first with stub `services/model-gateway/` (StubResolver) and `services/prompt-template-service/` (StubRenderer), plus a workflow-runtime LLM executor that calls them. F0b's job is now to **upgrade these stubs to production-grade without breaking the contract** that workflow-runtime's LLM executor expects:

- `model-gateway` `/v1/resolve` is consumed by `HTTPModelResolver` — preserve request/response shape.
- `prompt-template-service` `/v1/render` is consumed by `HTTPPromptRenderer` — preserve.
- `workflow-runtime`'s `HTTPLLMProvider` currently calls LiteLLM directly; F0b migrates it through `model-gateway`'s new `/v1/chat` endpoint.

**Action.** F0b acceptance includes running the `ai-email-triage` reference flow end-to-end with the upgraded gateway and confirming no regression.

### F7 ↔ F0c cost telemetry contract

F7 (billing) ingests cost data from F0c (model-gateway routing + cost) plus GCP Billing APIs. The contract — fields, granularity, latency — must be agreed before either ships in production.

### F8 ↔ F3 cluster bootstrap

F8 creates per-tenant GCP Projects; F3 runs cluster bootstrap inside those projects. Handoff (Project credentials, IAM trust, region selection) must be designed jointly.

### F6 ↔ F5 iteration UX

F6 (codebase-aware Alfred) provides the iteration capability; F5 surfaces it. F5 must define "select app → start chat scoped to it" before F6 builds the codebase-reading. Coordinate before either implementation.

### F9 + F10 ↔ F5

F9 (autopilot) is a flag that F5 reads and F10 renders. Three fronts touch it. Define the contract early.

### F1b ↔ `prompt-registry` extensions

F1b extends `prompt-registry` (tenant scope, `/validate-output`, `/v1/invoke`) — small (~6-9h) but the registry has no production consumers, so changes will not be detected by existing integration tests. F1b should add a smoke test from at least one SDLC skill end-to-end.

### Track B ↔ archived `ai-flow-authoring` custom node SDK v0

`ai-flow-authoring` (archived 2026-05-16) shipped custom node SDK at v0: `docs/sdk/custom-nodes.md` contract, example uppercase-text node, `/admin/custom-nodes/` form. Track B extends with publish API, marketplace, sandboxing, signing. Track B must not break v0 — backward compatibility required. workflow-runtime's `stubActivity` for `StepCustom` returns `step_type_not_yet_implemented` today; Track B replaces this with the production JWT-signed endpoint dispatcher specified in `docs/sdk/custom-nodes.md`.

### Product B ↔ Decision 17 "deploy as API endpoint"

The user clarified that Product B workflows should support being deployed as managed HTTP endpoints (Flowise pattern). This is an `ai-flow-authoring` track concern, not the umbrella's, but flag for that team to confirm it fits Phase B/E scope or warrants a follow-up.

## Impact

- **Code.** Each front carries its own diff. Cumulative footprint touches: most of `services/sdlc-*`, all of `services/model-gateway` (new), parts of `services/alfred/`, `services/app-onboarding/`, `services/deploy-orchestrator/`, `services/runtime-registry/`. New services: `services/iac-apply-runner/`, `services/iac-pr-driven-apply/` (Atlantis wrapper), `services/cloud-provisioner/`, `services/billing/`. Portal: new components and routes for F10.
- **APIs.** No API changes from the umbrella itself. Sub-changes document their own contracts.
- **Governance.** Each sub-change should reference back to this umbrella in its `## Why` section so the rationale chain is auditable.
- **Migration.** No migrations from the umbrella. Sub-changes carry their own.
- **Tests.** The umbrella's acceptance criteria become the integration test for F5 + F10. An end-to-end Playwright test should exercise intent → URL across the entire stack and live in the F5 change. F7's invoice acceptance has a separate integration test reading from a test tenant's accumulated usage.

## How this umbrella is closed

When F5, F6, F7, F8, F9, and F10 ship, and the acceptance criteria above pass on `main` for a test tenant, this change can be archived. Track B's `connector-lifecycle` does not need to ship to close the umbrella (different product surface), but it remains tracked in the repo's change log.

Decision 17's V2 fronts (F11 AWS, F12 composability, F13 intent-classification) are NOT required for closure.
