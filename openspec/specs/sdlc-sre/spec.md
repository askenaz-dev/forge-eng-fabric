# sdlc-sre Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: SRE skills

The capability SHALL expose `define-slos-from-spec`, `generate-runbook`, `tune-alerts` as registered skills.

#### Scenario: SLOs derived from OpenSpec NFRs

- **GIVEN** an OpenSpec containing non-functional requirements (latency, availability)
- **WHEN** Alfred invokes `define-slos-from-spec`
- **THEN** SLOs and SLIs MUST be created with target, window, error budget, and stored linked to the OpenSpec
- **AND** Prometheus rules MUST be proposed via PR

### Requirement: SRE gates

Gates `slos_defined`, `runbook_published`, `alerts_configured`, `on_call_assigned` MUST be evaluated before progression to `finops`.

#### Scenario: Block when no on-call assigned in production

- **GIVEN** an initiative deploying to prod with no on-call rotation
- **WHEN** progression is requested
- **THEN** gate `on_call_assigned` MUST fail
- **AND** Alfred MUST propose adding the Workspace's default on-call group
