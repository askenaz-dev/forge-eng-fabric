# SDLC Traceability

The traceability graph links SDLC artifacts across systems.

## Node Types

- `openspec`
- `jira_issue`
- `jira_epic`
- `confluence_page`
- `adr`
- `pr`
- `commit`
- `deployment`
- `test_run`
- `slo`
- `incident`
- `cost_record`

## Link Types

- `derives_from`
- `implements`
- `validates`
- `deploys`
- `monitored_by`
- `cost_attributed_to`

## Event Ingestion

The service consumes events such as `pr.linked-to-openspec.v1`, `deployment.applied.v1`, `jira.issue.updated.v1`, `confluence.page.updated.v1`, `test.run.completed.v1`, and `finops.budget.threshold_reached.v1`.

## Query

Use `GET /v1/traceability/{openspec_id}?depth=4` to retrieve the materialized subgraph rooted at an OpenSpec.

Materialized views refresh every 5 minutes and are exposed through `traceability_query_latency_p95` and `traceability_coverage` metrics.
