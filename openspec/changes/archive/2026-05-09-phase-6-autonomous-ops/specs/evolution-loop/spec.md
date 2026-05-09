# Spec Delta: evolution-loop (ADDED)

## ADDED Requirements

### Requirement: Generate OpenSpec change proposals from postmortems

On `postmortem.published.v1`, the loop MUST analyze the postmortem and produce an OpenSpec change proposal capturing concrete improvements (acceptance criteria, runbook updates, SLO adjustments, new gates, new healing actions).

#### Scenario: Proposal created with autonomous-loop marker

- **GIVEN** a postmortem with root cause "missing rate limit on /search"
- **WHEN** the loop runs
- **THEN** a change proposal MUST be created with `marker.source=autonomous-loop`
- **AND** referencing the postmortem URL
- **AND** containing concrete diffs to OpenSpec acceptance criteria
- **AND** emit `evolution.openspec_proposal.v1`

### Requirement: Human review required before adoption

Proposals MUST land in an "Evolution Inbox"; adoption requires explicit human approval converting the proposal into a normal OpenSpec change subject to existing workflows.

#### Scenario: Inbox approval converts to normal change

- **GIVEN** an evolution proposal `prop-7` in the inbox
- **WHEN** an approver accepts
- **THEN** a normal OpenSpec change MUST be created carrying the proposal contents
- **AND** the change MUST follow the standard lifecycle (in_review, approved, etc.)
- **AND** the proposal record MUST be marked `adopted_as=<change_id>`

### Requirement: Tracking metrics

The service MUST track proposals created, accepted, rejected, and time-to-accept; metrics surfaced in dashboards.

#### Scenario: Metrics visible in dashboard

- **GIVEN** 50 proposals in last 30 days, 20 adopted
- **WHEN** the dashboard renders
- **THEN** acceptance rate `40%`, median time-to-accept, and source-distribution by capability MUST be visible
