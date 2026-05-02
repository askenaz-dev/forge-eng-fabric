# Spec Delta: postmortem-generator (ADDED)

## ADDED Requirements

### Requirement: Auto-generate on incident resolution

On `incident.resolved.v1`, the service MUST generate a structured postmortem covering: summary, impact, timeline, root cause, what went well, what went wrong, action items.

#### Scenario: Postmortem generated and published

- **GIVEN** an incident `inc-1` resolved
- **WHEN** the generator runs
- **THEN** a postmortem document MUST be produced and published in Confluence in the Workspace's space
- **AND** linked from the OpenSpec of the affected asset
- **AND** emit `postmortem.generated.v1` and `postmortem.published.v1`

### Requirement: Action items linked to Jira

Action items in the postmortem MUST be created as Jira issues with owner, due date and link back to the postmortem.

#### Scenario: Action items materialized as issues

- **GIVEN** a postmortem with 3 action items each having an owner
- **WHEN** publishing completes
- **THEN** 3 Jira issues MUST exist with assignee = owner, label `postmortem-action-item`
- **AND** each issue body MUST contain the postmortem URL

### Requirement: Quality gate before close

The postmortem MUST pass an eval suite checking presence of all sections, citations to evidence, and action items with owners; failures MUST flag the postmortem as `needs-review` and notify the responsible team.

#### Scenario: Reject empty action items section

- **GIVEN** a draft postmortem missing action items
- **WHEN** the eval runs
- **THEN** the document MUST be flagged `needs-review` with reason `missing_action_items`
- **AND** notification sent to the Workspace owners
