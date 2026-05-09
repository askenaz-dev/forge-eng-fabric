# Spec Delta: sdlc-product (ADDED)

## ADDED Requirements

### Requirement: Product-phase skills available

The capability SHALL expose at minimum the skills `refine-user-story`, `generate-acceptance-criteria`, `prioritize-backlog`, registered as Registry assets with `lifecycle_state=approved` and `trust_level≥T2`.

#### Scenario: Refine user story produces structured output

- **GIVEN** a Jira issue with vague description
- **WHEN** Alfred invokes `refine-user-story` skill
- **THEN** the output MUST contain `as_a`, `i_want`, `so_that`, `acceptance_criteria[]`
- **AND** the skill MUST emit `skill.invoked.v1` with eval score
- **AND** the issue MUST be updated via Jira MCP with the structured content

### Requirement: Product-phase gates contract

The orchestrator MUST evaluate the gates `acceptance_criteria_present` and `story_size_estimated` before allowing progression to `architecture`.

#### Scenario: Block progression when criteria missing

- **GIVEN** an initiative in `phase=product` whose linked Jira issues lack acceptance criteria
- **WHEN** progression is requested
- **THEN** gate `acceptance_criteria_present` MUST fail
- **AND** emit `sdlc.phase.blocked.v1` with the offending issues listed
