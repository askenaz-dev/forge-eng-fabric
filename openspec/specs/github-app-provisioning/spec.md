# github-app-provisioning Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Short-lived installation tokens

The Forge GitHub App SHALL emit installation tokens scoped to a single repository with TTL ≤ 10 minutes for every mutation operation.

#### Scenario: Token scoped to target repo

- **GIVEN** an onboarding mutation targeting `org/svc-foo`
- **WHEN** the GitHub MCP requests an installation token
- **THEN** the token MUST be scoped to `org/svc-foo` only
- **AND** MUST expire in ≤ 10 minutes
- **AND** the token issuance MUST be audited

### Requirement: Standard branch protections

When a repo is created via Forge, branch protections on `main` MUST be applied automatically: PR required, ≥1 review (≥2 for `criticality≥high`), code-owners review, dismiss stale, required status checks (CI baseline + `forge/openspec-link`), linear history, signed commits required for `criticality≥high`.

#### Scenario: Critical app gets stricter protections

- **GIVEN** an onboarding with `criticality=critical`
- **WHEN** the repo is created
- **THEN** branch protection MUST require ≥2 reviews, signed commits, linear history, and restrict pushes to the Forge App and the `break-glass` team
- **AND** emit `branch_protection.applied.v1`

### Requirement: CODEOWNERS and PR/issue templates

Each repo SHALL be created with a CODEOWNERS file derived from Workspace owners and template defaults, and PR/issue templates that enforce OpenSpec linking and conventional structure.

#### Scenario: CODEOWNERS includes Workspace owners

- **GIVEN** Workspace `ws-1` with owners `@team-a` and `@alice`
- **WHEN** a repo is created in `ws-1`
- **THEN** CODEOWNERS MUST include `* @team-a @alice` plus template-specific paths

### Requirement: Override break-glass

Modifications to standard branch protections MUST require an approved policy override with TTL ≤ 24h, fully audited, and auto-reverted at expiration.

#### Scenario: Break-glass relax with auto-revert

- **GIVEN** an approved override `relax-branch-protection` with TTL=4h
- **WHEN** the override is applied
- **THEN** branch protections MUST be relaxed during the TTL window
- **AND** an event `policy.override.granted.v1` MUST be emitted
- **AND** at TTL expiration the standard protections MUST be re-applied automatically
- **AND** an event `policy.override.expired.v1` MUST be emitted
