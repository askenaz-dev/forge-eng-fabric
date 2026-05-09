# incidents-kb Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Vectorize and index resolved incidents

On `incident.resolved.v1`, the service MUST vectorize a structured summary (symptoms, timeline highlights, root cause, mitigation) and index it in the Tenant's Milvus collection.

#### Scenario: Incident indexed after resolution

- **GIVEN** incident `inc-1` resolved with root cause and mitigation recorded
- **WHEN** the indexer runs
- **THEN** an entry MUST be added to `incidents-kb-{tenant}` with metadata
- **AND** emit `kb.incident.indexed.v1`

### Requirement: Similarity search API

`POST /v1/kb/incidents/similar` MUST return top-K most similar incidents with similarity score, used by the diagnosis pipeline.

#### Scenario: Diagnosis queries similar incidents

- **GIVEN** a new incident on `app-foo` with symptoms "5xx spike + latency"
- **WHEN** the diagnosis pipeline queries the KB
- **THEN** the response MUST include similar past incidents within the same Tenant
- **AND** results MUST exclude incidents from other Tenants

### Requirement: Recurrent incident clustering

A nightly clustering job MUST identify groups of similar incidents and emit `incident.recurrent.detected.v1` for groups exceeding a threshold (default ≥3 within 30 days).

#### Scenario: Recurrent cluster detected

- **GIVEN** 4 similar incidents in 28 days for `app-foo`
- **WHEN** the clustering job runs
- **THEN** an event `incident.recurrent.detected.v1` MUST be emitted
- **AND** an action item MUST be proposed to add a permanent fix in the OpenSpec evolution loop
