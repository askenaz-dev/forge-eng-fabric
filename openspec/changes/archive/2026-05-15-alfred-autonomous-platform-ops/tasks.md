## 1. Iter 1 — Symptom bus + passive observation

- [x] 1.1 Create Kafka topic `forge.symptoms.v1` in `deploy/compose` and the Helm chart with retention=24h, partitions=12, key=`fingerprint`; add DLQ topic `forge.symptoms.v1.dlq`
- [x] 1.2 Define and version the symptom event JSON schema under `contracts/events/symptoms.v1.schema.json`; publish Go/Python/TS types in `contracts/generated/`
- [x] 1.3 Document the fingerprint vocabulary (required + optional dimensions) in `docs/alfred/symptom-fingerprint.md`
- [x] 1.4 Scaffold `services/symptom-emitter-logs` (Go): Loki tail config, regex/match rules, sanitiser (ANSI strip, length cap, escape `<>`), Kafka producer, local buffer with overflow drop counter
- [x] 1.5 Scaffold `services/symptom-emitter-probe` (Go): probe registry YAML (HTTP/TCP/gRPC), 30s default cadence, state-change debounce, recovery events
- [x] 1.6 Scaffold `services/symptom-triager` (Go): consumer group `forge-symptom-triager`, schema validator, DLQ producer, structured-log-only triage decisions (no session spawning yet)
- [x] 1.7 Create migrations `db/migrations/symptom-triager/0001_init.sql`: tables `noise_rule`, `circuit_breaker_state`, `symptom_session` (all empty at this iter)
- [x] 1.8 Add unit tests for fingerprint canonicalisation (sort dimensions, reject unknown ones) and for the sanitiser (ANSI, length, escaping)
- [x] 1.9 Wire all three services into `deploy/compose/docker-compose.yaml` with health checks
- [x] 1.10 Add Prometheus scrape and Grafana panel: events/sec by emitter, DLQ rate, fingerprint top-N (visible from day one)
- [ ] 1.11 Acceptance: navigate the local stack for one hour; review the triager's structured-log decisions; confirm top-N fingerprints match what a human observer would flag

## 2. Iter 2 — Risk classifier policy and diagnostic-only platform-ops

- [x] 2.1 Author `policies/alfred/risk-classifier.rego` per design D6: pure function with the full input/output schema; reject Rego compile errors in CI
- [x] 2.2 Author `policies/alfred/self-protection.rego` per design D10 with build-time override-relaxation rejection
- [x] 2.3 Add `policies/alfred/overrides/` directory with a sample tenant override (e.g., `acme.rego`) demonstrating only-stricter-narrowing
- [x] 2.4 Add `tools/policy/lattice-check.sh` invoked by the OPA bundle pipeline to detect any override that relaxes a global rule; CI fails on violation
- [x] 2.5 Sign and publish the `alfred` bundle through the existing OPA bundle pipeline; record the `policy_bundle_hash` mechanism
- [x] 2.6 Scaffold `services/platform-ops` (Go) with: structured-log middleware, OPA evaluator client (with bundle-hash capture), audit writer, OpenAPI registry
- [x] 2.7 Implement diagnostic endpoints: `GET /v1/diagnostics/probe`, `GET /v1/diagnostics/inspect/{service}`, `GET /v1/diagnostics/logs?fingerprint=...` (all read-only)
- [x] 2.8 Activate the triager session-spawning path: when triage rule (5) fires, call `POST /v1/agent-mode/sessions` with `actor=system:alfred, trigger_source=symptom, autonomy_preset="diagnose-then-propose"`
- [x] 2.9 Extend `alfred-agent-mode`: accept new fields (`trigger_source`, `actor`, `actor_session`, `symptom_id`, `playbook_id`, `parent_session_id`); enforce `forbidden_trigger_source` for non-triager callers
- [x] 2.10 Add OpenFGA bootstrap: principal `system:alfred`, capability groups (`alfred:platform-readonly`, `alfred:platform-operator`, `alfred:tenant-operator`); seed `system:alfred` with `platform-readonly` only at this iter
- [ ] 2.11 Acceptance: synthesise a known symptom (e.g., stop a service in compose); confirm the triager spawns a diagnose-only session; confirm the session emits a structured diagnosis without mutating anything

## 3. Iter 3 — Mutate-runtime endpoints + verification gate + circuit breaker

- [x] 3.1 Implement `POST /v1/services/{name}/restart`: input schema, OPA pre-check, action, post-validate probe (HTTP/healthz), audit row with policy_bundle_hash; idempotent revert via `?revert=<audit_event_id>`
- [x] 3.2 Implement `POST /v1/services/{name}/scale`: declared via OpenAPI metadata; OPA fields populated
- [x] 3.3 Implement `POST /v1/services/{name}/cordon` and `/uncordon` (Kubernetes only, no-op in compose)
- [x] 3.4 Implement `POST /v1/circuit-breakers/{fingerprint}/reset` (admin-only via OPA)
- [x] 3.5 Enforce per-step `expected_outcome` requirement in the executor for non-human-triggered sessions; emit `alfred.agent_mode.step_missing_probe.v1` and pause for HITL on absence
- [x] 3.6 Implement post-validate execution inside `platform-ops` endpoints; on failure with `reversibility ∈ {trivial,easy}` auto-rollback and respond 502
- [x] 3.7 Implement per-fingerprint circuit breaker in the triager: increment failures on session terminal-failed, open after 3 consecutive, cooldown 30min, HITL queue while open
- [x] 3.8 Extend `BudgetProbe` with per-fingerprint and per-hour aggregates queryable by triager and executor
- [x] 3.9 Add Portal "Autonomous activity" view: list of recent sessions with status, fingerprint, action, verification artifact, link to audit row
- [ ] 3.10 Acceptance: kill a service; verify the triager spawns a session, Alfred picks restart, OPA permits, restart endpoint runs probe, audit row appears in portal; force 3 failures and confirm breaker opens

## 4. Iter 4 — Sandboxes L0/L1, mutate-data, mutate-config, noise-rule lifecycle

- [x] 4.1 Implement `POST /v1/sandbox/spawn` and `/{id}/run`, `/{id}/destroy` for tier 0 (in-process dry-run) and tier 1 (docker single-service); enforce TTL=30min default
- [x] 4.2 Wire sandbox network policies (deny outbound to prod hostnames; mock-secret resolver returning `{value: ..., mock: true}`)
- [x] 4.3 Implement `POST /v1/migrations/dry-run`, `/v1/migrations/run`, `/v1/migrations/rollback` using existing goose tooling; require `sandbox_min_tier ≥ 1` per policy for non-trivial migrations
- [x] 4.4 Implement `POST /v1/feature-flags/{key}/toggle` and `/v1/secrets/{key}/rotate` with appropriate OPA decision and inverse actions
- [x] 4.5 Implement noise-rule endpoints: `POST /v1/noise-rules/propose`, `POST /v1/noise-rules/{id}/approve`, `POST /v1/noise-rules/{id}/revoke`; transactional row+PR creation via the existing GitHub App; GitHub webhook handler promotes on merge
- [x] 4.6 Portal: add Approvals Inbox entries for proposed noise rules with evidence samples (10 most recent matching events, sanitised excerpt); rate-limit visualised
- [x] 4.7 Extend audit schema: `triggered_by` (symptom_id), `session_id`, `policy_bundle_hash`, `approvers[]`, `verification`, `rollback_action_id`
- [ ] 4.8 Acceptance: write a benign migration via Alfred's proposal flow; observe L1 dry-run pass; observe migration applied with rollback path tested; observe a synthetic noise rule proposed by Alfred and approved by admin, with PR opened and merged

## 5. Iter 5 — Mutate-code endpoint + emitter-ci + dual-approval

- [x] 5.1 Implement `POST /v1/code-fixes/open-pr`: accepts repo, branch, file diff or set of patches, PR title, commit message, body, `expected_outcome` (CI green); opens PR via GitHub App; NEVER merges
- [x] 5.2 Scaffold `services/symptom-emitter-ci` (Go): GitHub Actions webhook receiver; normalise `check_run` and `workflow_run` failure events to symptom events with appropriate fingerprints
- [x] 5.3 Extend policies-and-approvals engine: dual-approval semantics (`any` or `dual`); approver-self-revocation window (default 60s)
- [x] 5.4 Portal: in the Approvals Inbox surface `triggered_by` symptom context for autonomous PRs; show admin-OR-owner approver path with visual indicator of who has approved
- [x] 5.5 Add admin command to install an "auto-fix opt-out" at workspace and tenant level: a workspace.policy field `alfred.auto_fix.enabled = true|false`, evaluated by OPA before any mutating action
- [ ] 5.6 Acceptance: stage a failing CI in a sample app; observe emitter-ci publishing symptom; observe Alfred analysing, opening a PR; observe approvals queue with symptom context and dual-approval flow

## 6. Iter 6 — Sandboxes L2/L3 + mutate-infra + repo-level opt-out

- [x] 6.1 Implement `POST /v1/sandbox/spawn` tier 2 (ephemeral k8s namespace) and tier 3 (full stack) on the platform's reserved sandbox cluster (decision per design Open Question); enforce TTL=30min
- [x] 6.2 Implement at least one `mutate-infra` endpoint as proof of concept (e.g., `POST /v1/runtimes/{id}/recreate`) per existing runtime-registry semantics; require dual approval per policy
- [x] 6.3 Implement repo-level opt-out: load `.forge/policy.yaml` from the app's repo on each evaluation; merge with workspace and tenant overrides through the lattice
- [x] 6.4 Extend `symptom-emitter-metrics` to query Prometheus for threshold-cross signals; document recommended baseline alerts
- [x] 6.5 Extend `symptom-emitter-webhook` for Linear / PagerDuty / Slack-mention webhooks
- [x] 6.6 Add FinOps panels: sandbox spend by tier and tenant; LLM spend by triager; circuit breaker open-time per fingerprint
- [ ] 6.7 Acceptance: simulate cross-service drift (e.g., contract mismatch between two services); observe Alfred propose a coordinated fix verified in an L2 sandbox; observe dual approval, rollout, and post-action verification

## 7. Cross-cutting hardening (in parallel)

- [x] 7.1 Implement sub-principal minting (`system:alfred:session:<uuid>`) per session with capability intersection; auto-revoke on terminal status within 60s
- [x] 7.2 Add guardrail layer enforcement of `self-protection.rego` before any tool dispatch; emit `guardrail.trip.v1` with `reason` taxonomy
- [x] 7.3 Add prompt-injection metric and review queue; auto-page security on >10 trips/hour
- [x] 7.4 Add evidence-block sanitiser at the LLM call boundary; ensure evidence is always wrapped in fenced blocks before reaching planner/executor
- [x] 7.5 Per-tenant LLM budget cap with default in config and override mechanism documented
- [x] 7.6 Bundle hash mismatch tests: every audit row's `policy_bundle_hash` resolves in the bundle registry; alerts on dangling hashes (indicates bundle GC drift)
- [x] 7.7 Document the "propose a new platform-ops endpoint" workflow as the official escape hatch in `docs/alfred/proposing-endpoints.md`
- [ ] 7.8 Chaos drill: temporarily disable platform-ops; confirm triager queues to HITL and pages on-call instead of spawning sessions for denylisted targets
