# Spec Delta: openspec-backbone (MODIFIED)

## MODIFIED Requirements

### Requirement: Decision log extended with Jira/Confluence/test/SLO entries

The OpenSpec `decision_log` SHALL accept entry types `jira_link`, `confluence_link`, `test_run_link`, `slo_link`, `incident_link`, `cost_record_link` in addition to the existing types.

#### Scenario: Jira link recorded on issue creation

- **GIVEN** an OpenSpec `spec-7` linked to initiative `init-1`
- **WHEN** Alfred creates Jira epic `ENG-100` referencing the OpenSpec
- **THEN** `spec-7.decision_log` MUST receive an entry `{type: jira_link, key: ENG-100, url: ..., created_by: alfred, at: ...}`
- **AND** the OpenSpec version MUST be bumped if mutability rules require

### Requirement: Linked artifacts namespaces

The `linked_artifacts` field SHALL support namespaces `jira:`, `confluence:`, `test:`, `slo:`, `incident:`, `cost:` in addition to existing namespaces.

#### Scenario: Linked artifacts queryable

- **GIVEN** `spec-7` with linked artifacts including `jira:ENG-100`, `slo:slo-12`, `confluence:DESIGN-42`
- **WHEN** the OpenSpec is fetched
- **THEN** `linked_artifacts` MUST list all entries with their namespace and external id
- **AND** the Portal viewer MUST render tabs grouped by namespace
