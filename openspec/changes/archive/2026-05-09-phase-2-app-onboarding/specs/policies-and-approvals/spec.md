# Spec Delta: policies-and-approvals (MODIFIED)

## MODIFIED Requirements

### Requirement: Onboarding policy templates

The policy engine SHALL ship with policy templates specific to onboarding: `require-approval-for-repo-creation`, `require-openspec-link`, `enforce-branch-protections`, `bypass-gate`, `merge-without-openspec`, `relax-branch-protection`, `allow-force-push`. Templates MUST be parameterizable per Workspace and criticality.

#### Scenario: Apply default onboarding policies on Workspace creation

- **GIVEN** a new Workspace is created in Phase 2
- **WHEN** default policies are applied
- **THEN** `require-openspec-link` MUST be active for `criticality≥medium`
- **AND** `enforce-branch-protections` MUST be active for all criticalities
- **AND** the policies MUST be visible in the Portal

### Requirement: Override TTL and audit

All onboarding-related overrides MUST have an explicit TTL (≤24h), be approved by a `security-approver` or `release-manager`, be auto-reverted at expiration, and be fully audited via `policy.override.granted.v1`, `policy.override.consumed.v1`, `policy.override.expired.v1`.

#### Scenario: Override expires and reverts

- **GIVEN** an approved override `relax-branch-protection` with TTL=4h applied to `main`
- **WHEN** 4 hours elapse
- **THEN** the engine MUST re-apply the standard branch protections
- **AND** emit `policy.override.expired.v1` with override id and revert details

#### Scenario: Reject override without approver role

- **GIVEN** a requester without `security-approver` or `release-manager` role
- **WHEN** they attempt to grant an override
- **THEN** the engine MUST refuse with `403 insufficient_role`
- **AND** record the denial in audit
