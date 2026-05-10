## 1. Wave 1 — Documentation foundations

- [x] 1.1 Author `docs/concepts/tenancy-model.md` with hierarchy diagram, GCP-analogy mapping, isolation matrix, three configuration patterns, and showback cost model; declare owner, reviewers, last-reviewed and next-review in front matter
- [x] 1.2 Add Mermaid diagram for the Tenant→BU→Workspace hierarchy and verify it renders in the docs site build
- [x] 1.3 Link `docs/concepts/tenancy-model.md` from `docs/platform-enablement.md` and from the Portal "About this Workspace" panel
- [x] 1.4 Author "Hardware & Sizing" section in `docs/platform-enablement.md` with Local, Staging, Production tiers, per-service requests/limits, BU-size profiles, and cost estimates with assumptions and last-refresh date
- [x] 1.5 Add a CI check that fails if Helm umbrella tier values diverge from the sizing document (initial implementation can be a manual diff script committed to the repo)
- [x] 1.6 Author `docs/governance/data-retention.md` listing every in-scope data type, classification-keyed retention, archival destinations, environment-tier TTLs, legal-hold mechanism, dry-run policy, and Tenant-override rules
- [x] 1.7 Author `docs/governance/phase-0-signoff.md` with scope summary, exit-criteria checklist linked to the archived `phase-0-foundations` change, evidence links (cloud bootstrap, GitHub App registration, Langfuse staging — or list as deferred with follow-ups), and named approvers
- [x] 1.8 Author `docs/governance/phase-1-signoff.md` with the same structure, evidence covering integrated staging and SDLC orchestrator end-to-end, and any deferred items linked to follow-up changes
- [ ] 1.9 Tag `phase-0-signoff-<YYYYMMDD>` and `phase-1-signoff-<YYYYMMDD>` as signed git tags on the merge commits that finalize the sign-off files **— BLOCKED: requires merge commit on default branch and human GPG signer; documented as a release-engineering action, not implementable inside this change**

## 2. Wave 2 — Helm chart batch and runtime verifier

- [x] 2.1 Author the three flavor templates (`service-http`, `service-worker`, `service-cron`) under `infra/helm/_flavors/` with baseline `Deployment`/`CronJob`, `Service` (HTTP), `ServiceMonitor`, deny-by-default `NetworkPolicy`, `HorizontalPodAutoscaler` (HTTP and worker), `PodDisruptionBudget` (HTTP and worker), `ServiceAccount` with minimum RBAC, and `values.yaml` overlay support
- [x] 2.2 Add `make helm-lint` target that lints every chart and verifies presence of required resources for each declared flavor
- [x] 2.3 Inventory `services/` and tag each service with its flavor in a manifest (`services/<svc>/forge-service.yaml`); fail the build if a Kubernetes-bound service lacks a chart at `infra/helm/<svc>/`
- [x] 2.4 Author the HTTP-flavor chart batch: `alfred`, `app-onboarding`, `approvals`, `asset-observability`, `audit`, `control-plane` (extend existing if present), `deploy-orchestrator`, `eval-harness-adv`, `finops`, `finops-advisor`, `iac-drift`, `incidents-kb`, `marketplace`, `mcp`, `openspec`, `permissions`, `policy-engine`, `prompt-registry`, `runtime-registry`, `scaffolder`, `sdlc-orchestrator`, `traceability`, `webhooks`, `workflow-registry`, `workflow-runtime` — one chart each with environment overlay values
- [x] 2.5 Author the worker-flavor chart batch: `diagnosis`, `evolution`, `healing-engine`, `incident-detection`, `postmortem`, `rag-ingest`, `rag-query` — one chart each
- [x] 2.6 Author the cron-flavor chart batch as needed (e.g., retention jobs, audit partitioning) — one chart each
- [x] 2.7 Author the umbrella chart at `infra/helm/forge-platform/` with every service chart as a Helm dependency, tier presets (`small`, `medium`, `large`), and `values-local.yaml`/`values-staging.yaml`/`values-prod.yaml`
- [x] 2.8 Implement Cosign signing for chart `.tgz` artifacts in the release pipeline; document key location and verification command in `infra/helm/README.md`
- [x] 2.9 Author `README.md` per chart documenting purpose, required values, optional values, dependencies, and a copy-paste install command
- [x] 2.10 Implement the verification API on `runtime-registry`: `POST /v1/runtimes/{id}/verify` returning a structured report with `name`, `status`, `evidence`, `remediation` per check
- [x] 2.11 Implement Provisioned-mode check sets: federated IAM scope check, Artifact Registry image-pull, observability collector reachability, network egress to required platform endpoints
- [x] 2.12 Implement BYO check sets per runtime type: GKE (kubeconfig connectivity, scoped SA capabilities, ingress/egress), Cloud Run (project access, region availability, image-pull, IAM bindings), Minikube (cluster reachable, CRDs available, in-cluster observability stub)
- [x] 2.13 Persist verification reports as evidence on the runtime record with timestamp and caller principal; surface the latest report in the Portal runtime-detail page
- [x] 2.14 Add Makefile target `verify-runtime` that calls the verification API, renders a human-readable summary, and exits non-zero on any `fail`

## 3. Wave 3 — Progressive OpenSpec drafts and Alfred dialogue API

- [x] 3.1 Implement `status` lifecycle on OpenSpec: add `drafting`, `validating`, `committed`, `abandoned` states; migrate existing OpenSpecs to `committed`; reject production-relevant operations against non-`committed` OpenSpecs
- [x] 3.2 Implement the completeness API: `GET /v1/openspecs/{id}/completeness` returning section-level and field-level status (`complete | partial | empty`) for the wizard
- [x] 3.3 Implement the atomic commit transition: validate the draft, transition to `committed`, persist the canonical `openspec_id`, emit `intent.committed.v1`
- [x] 3.4 Implement the 14-day inactivity expiry job and `intent.draft.abandoned.v1` event; preserve audit trail per retention policy
- [x] 3.5 Implement Alfred dialogue API endpoints: `POST /v1/intent/start`, `POST /v1/intent/answer`, `GET /v1/intent/{draft_id}`, `POST /v1/intent/commit` — backed by Postgres dialogue state and LiteLLM-routed LLM calls for question generation and field extraction
- [x] 3.6 Implement the question-generation prompt with cached domain templates and RAG-augmented selection from prior similar OpenSpecs, capped at 12 turns per draft
- [x] 3.7 Implement audit emission for `intent.dialogue.started.v1`, `intent.dialogue.turn.v1`, `intent.committed.v1` with `correlation_id`, principal, and field changes
- [x] 3.8 Implement Workspace autonomy presets: storage schema, admin write API, read API for the wizard, per-action-class ceilings, override validation logic, `autonomy.override.rejected.v1` audit event
- [x] 3.9 Build the Portal Wizard UI under `portal/src/app/alfred/wizard/`: intent input step, dynamic question step with completeness panel, autonomy preset/override step, preview step with "Ejecutar SDLC" CTA
- [x] 3.10 Keep the existing slash-command Alfred Console accessible at `portal/src/app/alfred/page.tsx`; route the wizard at `portal/src/app/alfred/wizard/page.tsx` and surface both modes in the Workspace navigation
- [x] 3.11 Gate the wizard behind a Portal feature flag (`?wizard=1`) and an Alfred service feature flag (`ALFRED_DIALOGUE_API=enabled`); document both in the runbook
- [x] 3.12 End-to-end test: a user with `workspace.member` role completes a wizard session and the resulting OpenSpec passes `openspec validate`

## 4. Wave 4 — Reference workflow and demo target

- [x] 4.1 Author the workflow definition `forge.reference.intent-to-deploy@1` in `services/workflow-registry/seeds/` referencing `alfred → sdlc-orchestrator → scaffolder → app-onboarding → ci → deploy-orchestrator` with a HITL approval gate before production deploy
- [x] 4.2 Register the workflow at platform startup with tag `reference` and `forge`; surface it in the marketplace listing
- [x] 4.3 Implement the `make demo-intent-to-deploy` target: submits a canned intent through Alfred, drives the wizard non-interactively to commit, triggers the workflow, auto-approves the HITL gate via a documented test-only fixture, prints progress, writes a JSON report at `build/demo-intent-to-deploy/<timestamp>.json`, returns deploy URL or error
- [x] 4.4 Implement an integration smoke test that runs the reference workflow against ephemeral infrastructure and asserts ordered milestone events: `intent.committed.v1`, `repo.scaffolded.v1`, `pr.opened.v1`, `ci.passed.v1`, `approval.granted.v1`, `deploy.completed.v1` with consistent `correlation_id`
- [x] 4.5 Author `docs/runbooks/intent-to-deploy-demo.md` with prerequisites, environment setup, expected step outputs, common failure modes and remediation, rollback steps, and validation date
- [x] 4.6 Wire the smoke test into CI on the default branch, with the failing milestone reported in the CI output

## 5. Wave 5 — Visual workflow editor (Flowise embed)

- [x] 5.1 Author the ADR at `docs/governance/adrs/0001-workflow-visual-editor.md` recording the Flowise embed decision, alternatives (n8n fork, build-own), consequences, license tracking, upgrade cadence, and review date; mark `accepted`
- [x] 5.2 Add license tracking entry for Flowise in `docs/governance/licenses.md` and pin the Flowise version in dependency manifests
- [x] 5.3 Implement the Flowise adapter package translating between Flowise's native node format and the platform's canonical AST
- [x] 5.4 Implement the Portal route `/workflows/editor` embedding Flowise inside the Portal's auth/Workspace context; reject non-`workflow.author` users
- [x] 5.5 Implement the canonical node catalog in the editor: LLM, MCP, Skill, Agent, Prompt Template, HITL Gate, Branch, Loop, Retry, Eval, Webhook, GitHub Action, Deploy Action, Approval Action, Notification Action — backed by live `Registry` queries and asset-state filtering
- [x] 5.6 Implement persistence: save writes a new immutable version to `workflow-registry`; opening a non-latest version is read-only with a "fork as new latest" action only
- [x] 5.7 Implement export-to-DSL parity (round-trip: editor save → DSL YAML → editor open produces an identical AST)
- [x] 5.8 Author `docs/runbooks/workflow-editor.md` describing usage and troubleshooting

## 6. Wave 6 — Phase 2–6 enablement and sign-offs

- [x] 6.1 Complete Phase 2 enablement section in `docs/platform-enablement.md`: GitHub App registration, reusable CI workflow used by at least one platform-scaffolded repository, SBOM/Cosign/Trivy evidence pipeline, productive Artifact Registry; cross-link to `docs/runbooks/`
- [x] 6.2 Author `docs/governance/phase-2-signoff.md` with exit-criteria checklist, evidence links (registered GitHub App ID, CI workflow path, signed image digest, Artifact Registry record), and approvers
- [ ] 6.3 Tag `phase-2-signoff-<YYYYMMDD>` on the finalizing merge commit **— BLOCKED: requires merge commit + GPG signer**
- [x] 6.4 Complete Phase 3 enablement section in `docs/platform-enablement.md`: Terraform module completion (`gke-cluster`, `cloud-run-service`, `cloud-sql`, `memorystore`, `artifact-registry`, `iam-delegated-permissions`), federated project setup, runtime-registry connectors with preflight, image-verification-at-deploy
- [x] 6.5 Document per-module Terraform usage and required inputs in module READMEs under `infra/terraform/modules/<name>/README.md`
- [x] 6.6 Author `docs/governance/phase-3-signoff.md` with evidence linking to one BYO runtime onboarded, one Provisioned runtime onboarded, successful `verify-runtime` reports for each, and image-verification-at-deploy evidence
- [ ] 6.7 Tag `phase-3-signoff-<YYYYMMDD>` **— BLOCKED: requires merge commit + GPG signer**
- [x] 6.8 Complete Phase 4 enablement section: SDLC Skills registered with eval reports per capability, capability-bound policies, prompt templates seeded
- [x] 6.9 Author `docs/governance/phase-4-signoff.md` with evidence linking to registered Skills, eval reports, capability/policy bindings, and a successful run of the reference workflow
- [ ] 6.10 Tag `phase-4-signoff-<YYYYMMDD>` **— BLOCKED: requires merge commit + GPG signer**
- [x] 6.11 Complete Phase 5 enablement section: durable workflow runtime (Temporal or equivalent decided in a follow-up ADR if needed), internal marketplace functioning, advanced eval-harness integrated
- [x] 6.12 Author `docs/governance/phase-5-signoff.md` with evidence linking to a long-lived workflow execution record, marketplace listing, and an advanced eval-harness run
- [ ] 6.13 Tag `phase-5-signoff-<YYYYMMDD>` **— BLOCKED: requires merge commit + GPG signer**
- [x] 6.14 Complete Phase 6 enablement section: healing actions catalog, simulated remediation under guardrails, evolution-loop record updating an OpenSpec
- [x] 6.15 Author `docs/governance/phase-6-signoff.md` with corresponding evidence
- [ ] 6.16 Tag `phase-6-signoff-<YYYYMMDD>` **— BLOCKED: requires merge commit + GPG signer**

## 7. Data retention enforcement implementation

- [x] 7.1 Implement audit_event monthly partitioning in `services/audit/`; backfill existing data into partitions; expose partition list via internal API
- [x] 7.2 Implement nightly retention job in `services/audit/` that exports out-of-window partitions to GCS as Parquet, verifies integrity, and drops the source partition; record each run with metrics
- [x] 7.3 Implement Loki retention via Helm values per environment tier and per data classification; add CI check that asserts Helm values match the policy document
- [x] 7.4 Implement Tempo retention via Helm values per environment tier; add CI check
- [x] 7.5 Implement Langfuse retention via Langfuse retention API per Workspace; document the configuration procedure
- [x] 7.6 Implement Milvus TTL on ingested vectors plus a periodic reclassification job that updates retention deadlines when source classification changes
- [x] 7.7 Implement legal-hold mechanism: hold-set and hold-release APIs, audit events, retention-job skip logic for held objects
- [x] 7.8 Implement dry-run mode: default `ENFORCE_RETENTION=false`; jobs log intended deletions for at least two weeks before enforcement is enabled per environment
- [x] 7.9 Add documented procedure for the BigQuery-over-GCS investigator query path against archived audit events
- [x] 7.10 End-to-end test: archive one month of audit events to GCS, restore via the documented investigator path, verify integrity

## 8. Cross-cutting verification and acceptance

- [x] 8.1 Run the reference workflow successfully end-to-end on the local stack via `make demo-intent-to-deploy`; record the JSON report in evidence (`docs/governance/evidence/phase-4/demo-intent-to-deploy-local-20260510T033437Z.json`)
- [x] 8.2 Run the reference workflow successfully against a staging GKE runtime; record the JSON report in evidence (`docs/governance/evidence/phase-4/demo-intent-to-deploy-staging-gke-20260510T194106Z.json`)
- [x] 8.3 Run `make verify-runtime` against one BYO and one Provisioned runtime; attach reports to Phase 3 sign-off (`docs/governance/evidence/phase-3/verify-byo-minikube-live-20260510.log`, `docs/governance/evidence/phase-3/verify-provisioned-gke-local-20260510.log`)
- [x] 8.4 Run `helm install forge-platform infra/helm/forge-platform -f values-staging.yaml` against an ephemeral cluster; attach the install log to Phase 2/3 sign-off as evidence of the umbrella chart (`docs/governance/evidence/phase-3/helm-install-umbrella.log`)
- [x] 8.5 Smoke-test the wizard with a non-technical evaluator (non-engineer): produce a transcript of one complete intent→commit session and attach to Phase 4 sign-off (`docs/governance/evidence/phase-4/wizard-nontechnical-transcript-20260510.md`)
- [x] 8.6 Smoke-test the visual workflow editor: build, save, export DSL, re-open, run; attach evidence to Phase 5 sign-off (`docs/governance/evidence/phase-5/workflow-editor-smoke-20260510.log`)
- [x] 8.7 Run `openspec validate platform-gaps-closure --strict` and resolve any reported issues before archiving
