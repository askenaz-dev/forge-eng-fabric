## ADDED Requirements

### Requirement: OPA policy maps action descriptor to autonomy decision

The platform SHALL ship a Rego policy `policies/alfred/risk-classifier.rego` that is a pure function from an action descriptor to a decision document. Input fields: `action_class ∈ {diagnostic, mutate-runtime, mutate-config, mutate-data, mutate-code, mutate-infra}`, `blast_radius ∈ {process, single-service, single-workspace, single-tenant, platform-wide}`, `reversibility ∈ {trivial, easy, hard, irreversible}`, `scope.{tenant_id, workspace_id, asset_id}`, `actor`, `trigger_source`. Output fields: `autonomy_decision ∈ {allow, requires_approval, requires_dual_approval, deny}`, `sandbox_min_tier ∈ {0,1,2,3}`, `approvers ∈ {admin, owner, dual}`, `reason` (string explaining the decision), `policy_bundle_hash`.

#### Scenario: Diagnostic actions are always allowed

- **WHEN** the policy receives `{action_class:"diagnostic"}` with any blast radius
- **THEN** it SHALL return `autonomy_decision:"allow"`, `sandbox_min_tier:0`

#### Scenario: Irreversible data action requires dual approval

- **WHEN** the policy receives `{action_class:"mutate-data", reversibility:"irreversible"}`
- **THEN** it SHALL return `autonomy_decision:"requires_dual_approval"`, `sandbox_min_tier ≥ 1`, `approvers:"dual"`

#### Scenario: Platform-wide mutating action requires admin

- **WHEN** the policy receives any `mutate-*` action with `blast_radius:"platform-wide"`
- **THEN** it SHALL return `autonomy_decision:"requires_approval"` with `approvers:"admin"` at minimum

#### Scenario: Mutate-code targeting an app routes to app owner

- **WHEN** the policy receives `{action_class:"mutate-code", blast_radius:"single-tenant"}` and `scope.asset_id` resolves to a non-platform asset
- **THEN** it SHALL return `autonomy_decision:"requires_approval"`, `approvers:"admin|owner"` (either suffices)

### Requirement: Override lattice — only stricter

Tenants and workspaces MAY provide override policy documents that *narrow* autonomy decisions (e.g., `autonomous → requires_approval`) but MUST NOT broaden them. The rego policy SHALL enforce this lattice using a canonical strictness order: `allow < requires_approval < requires_dual_approval < deny`.

#### Scenario: Tenant override tightens decision

- **WHEN** a tenant override declares `mutate-config` requires_approval and the global policy says `allow` for that input
- **THEN** the effective decision SHALL be `requires_approval`

#### Scenario: Tenant override attempts to relax — rejected

- **WHEN** a tenant override declares `mutate-data, reversibility:irreversible` is `allow` and the global policy says `requires_dual_approval`
- **THEN** the override SHALL be rejected at bundle build time
- **AND** the bundle pipeline SHALL fail with `code=override_relaxes_global` referencing the offending rule

### Requirement: Policy bundle is signed and versioned

The `policies/alfred/*` bundle SHALL be built, signed and distributed via the existing OPA bundle pipeline. The `policy_bundle_hash` returned by every evaluation SHALL match the published bundle's manifest hash and SHALL be persisted on every audit row of an autonomous action.

#### Scenario: Hash reconciliation

- **WHEN** an auditor inspects an audit row for an autonomous action
- **THEN** the row's `policy_bundle_hash` SHALL be lookup-able in the bundle registry, returning the exact Rego sources, override files, and signing certificate that produced the decision

### Requirement: Policy decisions are auditable and deterministic

The policy SHALL be a pure function: identical inputs in the same bundle version SHALL produce identical outputs. The decision document SHALL include a human-readable `reason` field referencing the rule(s) that produced the decision.

#### Scenario: Reason is populated

- **WHEN** the policy returns `requires_dual_approval` for an irreversible migration
- **THEN** the `reason` field SHALL contain a string like `irreversible mutate-data on single-tenant scope per rule risk-classifier.irreversible_data_requires_dual`
