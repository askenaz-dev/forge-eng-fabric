## Why

Today Alfred only acts when a human types an intent. The platform team, app owners and on-call still hand-operate the loop of *"watch logs → diagnose → fix"* — exactly what we walked through manually in the local stack: starting missing services, applying migrations, patching contract drift, silencing log noise, opening PRs. That loop is mechanical for ~70% of incidents (known noise, known runbook) and high-judgement for ~30% (novel symptoms, multi-service drift). Alfred already has the agent-mode planner/executor, autonomy presets, FGA-scoped tooling and audit chain to take this over — what is missing is the **non-human trigger path**, the **bounded action surface** for self-operations, the **sandbox-backed validation** before mutating shared state, and the **invariants** that keep an autonomous loop from cascading.

This proposal turns Alfred into the platform's autonomous operator: it observes the platform and the apps running on it, classifies what it sees against a versioned risk policy, validates fixes in tiered sandboxes, and either acts autonomously (low blast radius, reversible) or escalates to a human admin / app owner with full context. The aim is "Alfred-as-butler" — full autonomy where the risk is controlled, frictionless escalation where it isn't.

## What Changes

- **NEW** `forge.symptoms.v1` Kafka topic carrying normalized symptom events (one event per actionable signal) with a stable semantic `fingerprint` used for dedup, coalescing, circuit-breaking and noise rules.
- **NEW** A family of *symptom emitters* — small, single-responsibility services that translate raw signals (Loki tails, Prometheus thresholds, GitHub webhooks, periodic probes) into symptom events. Emitters DO NOT triage.
- **NEW** A *symptom triager* — the sole producer of non-human `AgentModeSession`s. Applies noise rules, coalescing, playbook matching and circuit-breaker checks before spawning sessions with `actor = system:alfred`.
- **NEW** `platform-ops` service with **dedicated narrow endpoints** for every action Alfred may take on the platform or on apps it monitors (restart service, scale, rotate secret, apply migration, toggle feature flag, open PR, spawn/destroy sandbox, propose noise rule, …). Generic shell access (kubectl, psql, docker) is **never** exposed to Alfred; the escape hatch is "Alfred proposes a new endpoint via PR".
- **NEW** An OPA-based risk classifier (`policies/alfred/risk-classifier.rego`) that maps `(action_class, blast_radius, reversibility, scope) → (autonomy_decision, sandbox_min_tier, approvers)`. Policy is global; tenants and workspaces MAY override only to be **stricter**.
- **NEW** Tiered sandbox primitives L0–L3 (dry-run, single-service container, ephemeral namespace, ephemeral full stack). OPA picks the minimum tier; Alfred MAY escalate, never de-escalate.
- **NEW** A verification gate enforced by `platform-ops`: every step that mutates state declares an `expected_outcome`, the gate evaluates it post-action, on failure the action is rolled back (if reversible) or escalated; a per-fingerprint circuit breaker opens after N failed sessions.
- **NEW** A noise-rule lifecycle: Alfred proposes rules into `noise_rule` table; admin approves in the existing approvals queue; approval atomically (1) activates the rule and (2) opens a PR to `policies/noise-rules.yaml` so git remains the source of truth. Revert is supported in both directions.
- **NEW** Self-protection denylist (non-negotiable, no admin override): Alfred MUST NOT take any action whose `target` resolves to `alfred | symptom-triager | platform-ops | opa | keycloak`. Outages of those components page a human.
- **MODIFIED** `alfred-agent-mode`: sessions accept `actor = system:alfred` and a non-human trigger source (`symptom_id`); step execution gains required pre-validate / post-validate / rollback hooks; budget probe is extended to per-fingerprint and per-hour caps.
- **MODIFIED** `alfred-control-plane`: clarifies Alfred's autonomy under non-human triggers and binds it to the risk-classifier policy.
- **MODIFIED** `policies-and-approvals`: adds the dual approver model (admin OR app-owner) for code-fix PRs originated by Alfred, and records `triggered_by = symptom_id` alongside `approved_by`.
- **MODIFIED** `agentic-guardrails`: adds prompt-injection mitigations for log-sourced evidence (sanitisation, fenced evidence blocks) and the self-protection denylist.

## Capabilities

### New Capabilities

- `autonomous-symptom-bus`: Kafka topic, event schema, fingerprint contract and retention policy for normalized platform signals consumed by Alfred.
- `autonomous-symptom-emitters`: Single-responsibility services that translate raw signals (logs, metrics, CI, webhooks, probes) into symptom-bus events without triage logic.
- `autonomous-symptom-triager`: The single producer of non-human `AgentModeSession`s — dedup, coalescing, noise-rule application, playbook matching, circuit-breaker enforcement.
- `platform-ops-service`: Dedicated narrow HTTP API exposing every action Alfred may take on the platform or apps; encodes server-side invariants, validation and rich audit.
- `risk-classifier-policy`: OPA policy mapping action class × blast radius × reversibility × scope to autonomy decision, sandbox minimum tier, and approver set.
- `tiered-sandboxes`: L0 (dry-run) through L3 (ephemeral full stack) ephemeral environments used to validate mutating actions before they touch shared state.
- `verification-gate`: Pre/post-validate + rollback + per-fingerprint circuit breaker invariants applied to every autonomous mutating step.
- `noise-rule-lifecycle`: Proposal → admin approval → git source of truth → revertible activation flow for log-noise silencing.
- `alfred-self-protection`: Non-negotiable denylist preventing Alfred from acting on its own infrastructure (triager, platform-ops, OPA, Keycloak).

### Modified Capabilities

- `alfred-agent-mode`: Sessions accept `actor = system:alfred` and a non-human trigger source; per-step pre-validate / post-validate / rollback hooks become mandatory; budget probe extended with per-fingerprint and per-hour caps.
- `alfred-control-plane`: Autonomy clauses extended to non-human triggers, bound to the risk-classifier policy and the self-protection denylist.
- `policies-and-approvals`: Adds the admin-OR-owner dual-approval flow for code-fix PRs and the `triggered_by = symptom_id` field on approval records.
- `agentic-guardrails`: Prompt-injection mitigations for log-sourced evidence and the self-protection denylist invariants.

## Impact

- **Services (new)**: `symptom-emitter-logs`, `symptom-emitter-metrics`, `symptom-emitter-ci`, `symptom-emitter-webhook`, `symptom-emitter-probe`, `symptom-triager`, `platform-ops`.
- **Services (modified)**: `alfred` (agent-mode session model, executor hooks, budget), `audit` (new event types, `triggered_by` link), `approvals` (dual-approver flow, link to symptom).
- **Schemas**: new tables `noise_rule`, `circuit_breaker_state`, `symptom_session`, `sandbox_run`; new audit event types `alfred.autonomous.*`.
- **Kafka**: new topic `forge.symptoms.v1` with 24h retention; existing `forge.events` unaffected.
- **Policy**: new package `policies/alfred/` containing `risk-classifier.rego`, `self-protection.rego`, `noise-rules.yaml`; OPA bundle pipeline gains a deploy stage for these.
- **OpenFGA**: new principal `system:alfred` with per-session sub-principals (`system:alfred:session:<uuid>`) and capability groupings (`alfred:platform-readonly`, `alfred:platform-operator`, `alfred:tenant-operator`).
- **Infra**: ephemeral sandbox spawning requires either a dedicated docker host budget (L1) or a Kubernetes namespace pool (L2/L3); decision and capacity defaults belong in `design.md`.
- **Cost / risk**: per-hour LLM budget cap on the triager to bound runaway storms; expected MTTR reduction for "stack-not-up" and "known-runbook" incidents from minutes to seconds; novel-symptom incidents still page humans but with full diagnostic context.
- **UX**: portal gains an "Autonomous activity" view (timeline of Alfred's autonomous actions, pending approvals, circuit-breaker state, noise-rule proposals); existing approvals queue gains symptom-linked rows.
- **Migrations path**: rolled out in six iterations (passive observation → diagnostic-only → restart-only → migrations + flags + noise → code PRs → ephemeral sandbox-backed cross-service fixes); each iteration is independently shippable and reversible.
