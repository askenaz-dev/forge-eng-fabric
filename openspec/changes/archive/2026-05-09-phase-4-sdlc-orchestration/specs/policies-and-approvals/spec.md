# Spec Delta: policies-and-approvals (MODIFIED)

## MODIFIED Requirements

### Requirement: SDLC gate policy templates

The policy engine SHALL provide templates for SDLC gates: `require-architecture-review`, `require-threat-model`, `require-test-coverage`, `require-perf-budget`, `require-security-clean`, `require-slo-defined`, `require-runbook`, `require-on-call`, `require-cost-estimate`, `phase-progression-bypass` (override).

#### Scenario: Templates available for new Workspaces

- **GIVEN** a Workspace created in Phase 4 or later
- **WHEN** policy templates are listed
- **THEN** SDLC gate templates MUST be available with criticality-aware defaults

### Requirement: Phase progression bypass override

`phase-progression-bypass` MUST require approval by `release-manager`, mandatory rationale, TTL ≤ 24h, single-use, and full audit.

#### Scenario: Single-use bypass cannot be reused

- **GIVEN** an approved bypass for `phase=qa` of `initiative-3`
- **WHEN** the bypass is consumed once
- **THEN** subsequent attempts to use it MUST fail with `403 override_already_consumed`
- **AND** emit `policy.override.exhausted.v1`
