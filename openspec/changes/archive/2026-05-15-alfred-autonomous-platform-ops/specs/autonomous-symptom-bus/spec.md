## ADDED Requirements

### Requirement: Dedicated Kafka topic for normalised symptoms

The platform SHALL expose a single Kafka topic `forge.symptoms.v1` carrying normalised operational symptom events. The topic SHALL be distinct from `forge.events` and have its own retention, partitioning, and consumer-group rules.

#### Scenario: Topic exists with required configuration

- **WHEN** the platform compose / Helm chart is brought up
- **THEN** the topic `forge.symptoms.v1` SHALL exist with retention=24h, partitions=12, cleanup.policy=delete, key=`fingerprint`
- **AND** `forge.events` SHALL be unaffected and continue serving its existing producers

#### Scenario: Topic is versioned in the name

- **WHEN** a future schema-incompatible revision is needed
- **THEN** a new topic `forge.symptoms.v2` SHALL be introduced and coexist with `v1`
- **AND** existing emitters MUST continue producing to `v1` until they are individually re-targeted

### Requirement: Symptom event schema is fixed and validated

Every event published on `forge.symptoms.v1` SHALL conform to a JSON schema that includes: `id`, `occurred_at`, `source.kind`, `source.emitter`, `source.service`, optional `source.tenant_id` / `source.workspace_id` / `source.asset_id`, `signal.fingerprint`, `signal.title`, `signal.severity`, optional `signal.evidence_ref`, `signal.evidence_excerpt`, and optional `policy_hints`. The triager SHALL reject events that fail schema validation and send them to a dead-letter topic `forge.symptoms.v1.dlq`.

#### Scenario: Valid event is accepted

- **WHEN** an emitter publishes an event satisfying the schema
- **THEN** the triager SHALL consume it and process it according to its triage rules

#### Scenario: Invalid event is dead-lettered

- **WHEN** an emitter publishes an event missing a required field or with an unknown `source.kind`
- **THEN** the triager SHALL NOT process the event for actions
- **AND** SHALL forward the original payload plus a validation error to `forge.symptoms.v1.dlq` with a metric `symptom_bus.dlq_total{reason}` incremented

### Requirement: Stable fingerprint contract

`signal.fingerprint` SHALL be a string of `<dimension>:<value>` pairs joined by `|`, sorted alphabetically by dimension. Required dimensions: `service`, `signal`. Optional dimensions are drawn from a documented enumerated vocabulary (`tenant`, `workspace`, `error_class`, `port`, `route`, `severity_class`). Events with unknown dimensions SHALL be dead-lettered.

#### Scenario: Well-formed fingerprint passes

- **WHEN** an emitter sends `signal.fingerprint = "error_class:ECONNREFUSED|port:8094|service:workflow-registry|signal:probe-failed"`
- **THEN** the triager SHALL accept the event and key its downstream behaviour on that fingerprint

#### Scenario: Unsorted or unknown dimensions are rejected

- **WHEN** an emitter sends `signal.fingerprint = "service:registry|foo:bar"`
- **THEN** the triager SHALL dead-letter the event with reason `unknown_dimension:foo`

### Requirement: Topic key drives partition affinity

The Kafka record key SHALL be the event's `signal.fingerprint`. The triager SHALL be a single consumer group `forge-symptom-triager` so that all events with the same fingerprint are processed by the same consumer instance, enabling in-process dedup and coalescing without a distributed lock.

#### Scenario: Same fingerprint routes to one consumer

- **WHEN** 1000 events with `fingerprint = "service:registry|signal:probe-failed"` are produced
- **THEN** all 1000 SHALL be consumed by the same triager replica
- **AND** the triager SHALL deduplicate them into at most one active session
