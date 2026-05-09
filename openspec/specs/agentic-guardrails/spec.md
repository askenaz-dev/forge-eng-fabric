# agentic-guardrails Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: System vs untrusted-context separation
The platform SHALL structure prompts so that **system instructions** are clearly separated from **untrusted context** (RAG, user input, external content). Untrusted context SHALL be tagged and SHALL NOT be allowed to alter system instructions.

#### Scenario: Injected instruction in retrieved context is ignored
- **WHEN** retrieved RAG content contains text instructing the model to perform a sensitive action
- **THEN** the guardrail flags it, the model receives the content as untrusted, and any tool call attempted as a result is blocked unless on the allowlist

### Requirement: RAG content sanitization
Content retrieved from RAG SHALL be sanitized: known prompt-injection patterns flagged, scripts/HTML stripped or quoted, and provenance signed/validated.

#### Scenario: Unsigned source rejected at retrieval
- **WHEN** retrieval returns a chunk whose `provenance_signed` is invalid
- **THEN** the chunk is excluded from the prompt, an event is emitted and ingestion source is flagged for review

### Requirement: Tool allowlists by trust level and classification
Tools/MCPs marked as sensitive SHALL be invocable only by callers/contexts on the configured allowlist (by trust_level, data_classification, environment, criticality).

#### Scenario: Off-allowlist tool call is blocked
- **WHEN** a caller off the allowlist tries to invoke a sensitive tool
- **THEN** the call is blocked at the guardrail layer and audited with `guardrail.trip.v1`

### Requirement: Output schema validation
Outputs that must conform to a JSON schema (Skills, Prompt Templates) SHALL be validated; non-conforming outputs SHALL trigger a guardrail-trip and be retried (per policy) or failed.

#### Scenario: Non-conforming JSON output is rejected
- **WHEN** a Skill returns JSON not matching its declared `outputs_schema`
- **THEN** the runtime rejects the output, emits `guardrail.trip.v1`, and retries up to the configured limit before failing

### Requirement: Guardrail-trip metrics
The platform SHALL expose metrics for guardrail trips by Workspace, asset, type and severity, with alerting thresholds configurable.

#### Scenario: Trip rate exceeds threshold for a Workspace
- **WHEN** a Workspace's guardrail-trip rate exceeds the configured SLO
- **THEN** owners and the SDLC Team are alerted
