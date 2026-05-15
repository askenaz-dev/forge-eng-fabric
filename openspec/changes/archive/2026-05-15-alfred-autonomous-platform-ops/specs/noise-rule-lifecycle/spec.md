## ADDED Requirements

### Requirement: Alfred proposes, admin approves, git is source of truth

Noise rules SHALL flow through a four-state lifecycle: `proposed` (by Alfred or admin) → `approved` (by admin via portal) → `promoted` (PR merged in git) → `revoked` (rule deactivated). The `noise_rule` table is the runtime source; `policies/noise-rules.yaml` in git is the source of truth. The two SHALL be reconciled by the approval transaction.

#### Scenario: Alfred proposes a rule

- **WHEN** the triager detects N (default 10) consecutive events of the same fingerprint that produced no actionable session in M (default 24h)
- **THEN** Alfred SHALL invoke `POST /v1/noise-rules/propose` with `{fingerprint_pattern, justification, evidence_sample_ids, ttl_days}`
- **AND** the rule SHALL be persisted with `state="proposed"` and appear in the admin portal queue

#### Scenario: Admin approval is atomic with PR creation

- **WHEN** an admin clicks `Approve` on a proposed rule
- **THEN** `platform-ops` SHALL, in a single transaction: set rule state to `approved`, set `approved_by` and `approved_at`, and open a PR to `policies/noise-rules.yaml` referencing the rule's UUID
- **AND** if PR creation fails, the rule SHALL remain `proposed` and the admin SHALL see an error

#### Scenario: PR merge promotes the rule

- **WHEN** the PR is merged in GitHub
- **THEN** the GitHub webhook SHALL trigger a transition to `promoted` and populate `promoted_at` and `promoted_commit`

#### Scenario: PR closed without merge demotes the rule

- **WHEN** the PR is closed without merging
- **THEN** the rule SHALL be marked `draft` with `expires_at = now() + 7d` and a portal banner SHALL prompt for action

### Requirement: Revocation is symmetric

A rule MAY be revoked at any time by an admin. Revocation SHALL atomically deactivate the runtime row and open a revert PR against `policies/noise-rules.yaml`. The same atomicity rules apply: both happen, or neither does.

#### Scenario: Admin revokes a rule

- **WHEN** an admin invokes `POST /v1/noise-rules/{id}/revoke` with a reason
- **THEN** the row's `revoked_at` and `revoked_by` SHALL be set, the runtime SHALL stop matching, and a revert PR SHALL be opened against the rule's prior commit

#### Scenario: Revert PR merge finalises revocation

- **WHEN** the revert PR is merged
- **THEN** the rule's state SHALL transition to `revoked` with `revoked_commit` populated

### Requirement: Rules carry evidence and rate-limited proposal cadence

Each proposed rule SHALL include `evidence_sample_ids` (UUIDs of representative events from the bus) and `justification`. Alfred SHALL NOT propose more than N (default 5) new rules per workspace per day; excess proposals SHALL be coalesced or deferred.

#### Scenario: Evidence enables admin review

- **WHEN** an admin views a proposed rule
- **THEN** the portal SHALL display up to 10 sample events matching the rule's pattern, with timestamps, severity, and excerpt

#### Scenario: Proposal rate-limit

- **WHEN** Alfred has already proposed 5 rules for workspace X in the last 24h
- **THEN** the triager SHALL queue further proposal-eligible patterns and emit at most one summary proposal at the next cadence

### Requirement: Audit covers every state transition

Every transition `proposed → approved → promoted → revoked` (and intermediate `draft` states) SHALL produce its own `audit_event` row. Audit rows SHALL link the rule UUID, the actor (Alfred or admin), and the relevant PR / commit references.

#### Scenario: Full lifecycle is auditable

- **WHEN** a rule progresses through all states and is then revoked
- **THEN** at least four audit rows SHALL exist (one per transition) with linked `correlation_id`, and an auditor SHALL be able to reconstruct the full timeline with actor and reason
