## ADDED Requirements

### Requirement: Canonical data retention policy document

The platform SHALL publish `docs/governance/data-retention.md` as the canonical retention and lifecycle policy. The policy SHALL be keyed on data type and data classification and SHALL specify retention duration, archival destination, and deletion cadence per environment tier.

#### Scenario: Policy lists every in-scope data type

- **WHEN** a reviewer audits the policy
- **THEN** the policy SHALL include rows for: audit events, Langfuse LLM traces, RAG content (Milvus + source documents), platform logs (Loki), platform traces (Tempo), platform metrics (Prometheus), workflow execution history, and approval records

#### Scenario: Policy keyed on classification

- **WHEN** a reviewer audits the policy
- **THEN** every row SHALL specify retention for each data classification (`public`, `internal`, `confidential`, `restricted`)

#### Scenario: Policy lists archival destination

- **WHEN** a reviewer audits the policy
- **THEN** every row that exceeds operational retention SHALL specify the archival destination (e.g., GCS bucket, region, encryption posture) and the lifecycle rule applied to the archive

### Requirement: Audit event partitioning and archival

The `audit-service` SHALL partition `audit_event` records by month and SHALL archive partitions older than the operational retention window to GCS, then drop them from Postgres.

#### Scenario: Audit events older than retention archived

- **WHEN** the nightly retention job runs and a partition's age exceeds the operational retention window for its classification
- **THEN** the job SHALL export the partition to the configured GCS bucket as a signed Parquet file, verify the export's integrity, and drop the partition from Postgres

#### Scenario: Archived audit events queryable

- **WHEN** an authorized investigator needs to query archived audit events
- **THEN** the platform SHALL provide a documented procedure (e.g., BigQuery external table over GCS) that does not require restoring partitions to Postgres

### Requirement: Loki, Tempo, and Langfuse TTLs configured per tier

Loki, Tempo, and Langfuse retention SHALL be configured per environment tier and per data classification through Helm values, and the values SHALL be the source of truth for the policy.

#### Scenario: Helm values match policy

- **WHEN** the retention policy lists a TTL for Loki at the `staging` tier for `internal` data
- **THEN** the Helm values for Loki at `staging` SHALL declare exactly that TTL, and a CI check SHALL fail if the policy and values diverge

### Requirement: RAG content TTL and reclassification

Milvus collections and source documents ingested into the RAG knowledge base SHALL carry classification and ingestion timestamps. A periodic re-evaluation job SHALL reapply retention rules when a source's classification changes.

#### Scenario: Source reclassified, retention updated

- **WHEN** the source of an ingested document is reclassified from `internal` to `restricted`
- **THEN** the next reclassification run SHALL update the retention deadline for the corresponding Milvus vectors and the original document

### Requirement: Legal hold mechanism

The platform SHALL provide a legal-hold mechanism that pauses retention deletions for tagged objects regardless of policy expiry, with auditable hold-set and hold-release operations.

#### Scenario: Object placed on legal hold is preserved

- **WHEN** an authorized compliance principal places an object on legal hold
- **THEN** subsequent retention runs SHALL skip deletion of the object, the hold-set event SHALL be audited, and the object SHALL remain visible to authorized investigators

#### Scenario: Hold release resumes retention

- **WHEN** an authorized compliance principal releases the legal hold
- **THEN** the next retention run SHALL evaluate the object normally, the hold-release SHALL be audited, and any expired data SHALL be deleted on the next eligible run

### Requirement: Dry-run mode before enforcement

Retention enforcement SHALL run in dry-run mode by default. Each environment SHALL require an explicit `ENFORCE_RETENTION=true` flag (set via Helm values) before deletions occur, and dry-run logs SHALL be retained for at least two weeks before enforcement.

#### Scenario: Dry-run logs intended deletions

- **GIVEN** `ENFORCE_RETENTION` is unset or false
- **WHEN** a retention job runs
- **THEN** the job SHALL log every record it would delete, with classification and policy reference, and SHALL NOT delete any records

#### Scenario: Enforcement enabled per environment

- **WHEN** an operator enables enforcement for a specific environment
- **THEN** the enabling change SHALL include a link to at least two weeks of dry-run logs and SHALL be co-approved by Platform and Security

### Requirement: Tenant overrides bounded by platform minimums

Tenants MAY lengthen retention beyond the platform default for legal-hold or compliance reasons. Tightening retention below the platform minimum SHALL be rejected.

#### Scenario: Tenant lengthens retention

- **WHEN** a Tenant admin requests longer retention for `audit` data
- **THEN** the override SHALL be applied with audit, and the new retention SHALL be visible in the policy view for that Tenant

#### Scenario: Tenant attempts to shorten below minimum

- **WHEN** a Tenant admin requests retention shorter than the platform minimum
- **THEN** the request SHALL be rejected with a clear message citing the policy minimum
