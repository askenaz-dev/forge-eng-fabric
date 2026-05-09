# sdlc-architecture Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Architecture skills

The capability SHALL expose `propose-adr`, `evaluate-options`, `check-openspec-alignment` as registered skills with eval suites.

#### Scenario: ADR generated and stored

- **GIVEN** an initiative in `phase=architecture` with multiple options
- **WHEN** Alfred invokes `propose-adr`
- **THEN** an ADR MUST be created (in repo `docs/adr/` or Confluence) with status `proposed`
- **AND** linked to the OpenSpec via `decision_log`
- **AND** emit `sdlc.adr.proposed.v1`

### Requirement: Architecture gates

Gates `adrs_published`, `security_review_passed`, `openspec_updated` MUST be evaluated before progression to `design`.

#### Scenario: Block on missing security review for high criticality

- **GIVEN** an initiative with `criticality=high`
- **WHEN** the architecture phase has ADRs but no recorded security review
- **THEN** gate `security_review_passed` MUST fail with reason `security_review_missing`
- **AND** emit `sdlc.phase.blocked.v1`
