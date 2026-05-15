## ADDED Requirements

### Requirement: Non-human triggers bind autonomy to the risk-classifier policy

When Alfred acts without a human in the loop (`actor = system:alfred`, `trigger_source ∈ {symptom, playbook}`), every mutating action SHALL be authorised by `policies/alfred/risk-classifier.rego`. The session executor SHALL NOT consult workspace `autonomy_policy` directly for non-human-triggered actions; instead it SHALL pass the action descriptor to OPA and obey the returned `autonomy_decision` and `sandbox_min_tier`.

#### Scenario: Symptom-triggered restart consults OPA

- **WHEN** a symptom-triggered session reaches a `mutate-runtime` step targeting a single service
- **THEN** the executor SHALL evaluate the action through `risk-classifier.rego` and, on `allow`, proceed; on `requires_approval`, pause for HITL
- **AND** SHALL ignore any conflicting workspace-level autonomy preset (the workspace can only narrow, never relax — enforced by override lattice)

#### Scenario: Workspace override narrows the OPA decision

- **WHEN** OPA returns `allow` for a non-human-triggered action but the workspace has an override declaring `requires_approval` for that action class
- **THEN** the executor SHALL apply the stricter decision (`requires_approval`)

### Requirement: Self-protection denylist binds the platform agent

Alfred SHALL NOT perform any action whose target resolves to `alfred`, `symptom-triager`, `platform-ops`, `opa`, or `keycloak`, regardless of trigger source or admin approval. The denylist is enforced by `policies/alfred/self-protection.rego` at the top of the policy chain.

#### Scenario: Alfred cannot restart its own dependency

- **WHEN** a session attempts to invoke `POST /v1/services/keycloak/restart`
- **THEN** OPA SHALL deny via self-protection
- **AND** the step SHALL be marked `failed` with `reason:"self_protection_denylist"` and the session SHALL pause for HITL

### Requirement: Session sub-principal scopes capabilities per session

For every non-human-triggered session, the triager SHALL mint a sub-principal `system:alfred:session:<uuid>` in OpenFGA whose granted capabilities are the **intersection** of `system:alfred`'s standing grants and the capabilities justified by the originating symptom's `policy_hints`. The session executor SHALL act with this sub-principal, not with `system:alfred` directly.

#### Scenario: Sub-principal restricts to symptom-relevant capabilities

- **WHEN** the symptom carries `policy_hints=["service-down"]` for `workflow-registry`
- **THEN** the minted sub-principal SHALL grant only restart/inspect capabilities scoped to that service
- **AND** SHALL NOT grant `mutate-data` or `mutate-code` regardless of standing grants

#### Scenario: Sub-principal revoked on session close

- **WHEN** a session reaches terminal status (`completed`, `failed`, `aborted`, `resolved_externally`)
- **THEN** the sub-principal SHALL be revoked from OpenFGA within 60 seconds
- **AND** the revocation SHALL be audited

## MODIFIED Requirements

### Requirement: Autonomy by default within delegated permissions

Alfred SHALL operate **autonomously by default** for actions allowed by policy and within active delegated permissions. For **human-triggered** sessions the relevant policy is the Workspace/OpenSpec `autonomy_policy`. For **non-human-triggered** sessions (`actor = system:alfred`, `trigger_source ∈ {symptom, playbook}`) the relevant policy is `policies/alfred/risk-classifier.rego`; the override lattice ensures workspace settings can only narrow OPA's decision, never relax it. Restrictions and approvals SHALL only apply when explicitly required.

#### Scenario: Autonomous action proceeds without HITL

- **WHEN** an action's effective policy decision (workspace policy for human triggers, OPA for non-human triggers) is `autonomous`/`allow` and Alfred has the necessary delegated permission
- **THEN** Alfred executes the action and records the policy decision, the `policy_bundle_hash` (when OPA-evaluated), and the outcome

#### Scenario: Action requiring approval is paused

- **WHEN** an action requires approval per the effective policy
- **THEN** Alfred opens an approval request including the action descriptor and the policy decision, halts execution, and resumes only after the configured approver(s) decide

#### Scenario: Self-protection denial overrides any allow

- **WHEN** any policy evaluation would otherwise allow an action but the target is on the self-protection denylist
- **THEN** the action SHALL be denied; admin approval SHALL NOT lift the denial; the rejection SHALL be audited as a security-channel event
