# autonomous-symptom-emitters Specification

## Purpose
TBD - created by syncing change alfred-autonomous-platform-ops. Update Purpose after archive.

## Requirements

### Requirement: Emitters are single-responsibility normalisers

Each symptom emitter SHALL translate raw signals from exactly one source kind into events on `forge.symptoms.v1`. Emitters SHALL NOT perform triage, dedup, or apply noise rules — that is the triager's responsibility. The initial emitter set is `symptom-emitter-logs` (Loki tail), `symptom-emitter-metrics` (Prometheus query), `symptom-emitter-ci` (GitHub Actions webhooks), `symptom-emitter-webhook` (third-party webhooks: Linear, PagerDuty, etc.), `symptom-emitter-probe` (scheduled health probes).

#### Scenario: Log emitter normalises a recognised pattern

- **WHEN** `symptom-emitter-logs` reads a line matching `ECONNREFUSED.*:(\d+)` from `workflow-registry`
- **THEN** it SHALL publish an event with `source.kind="log"`, `source.service="workflow-registry"`, `signal.fingerprint` containing the matched port, and `signal.evidence_excerpt` set to the sanitised log line

#### Scenario: Emitter does not triage

- **WHEN** a log emitter recognises a pattern that is already covered by an active noise rule
- **THEN** it SHALL still emit the event — the triager is responsible for applying noise rules at consume time
- **AND** noise rule application MUST NOT happen at the emitter

### Requirement: Evidence sanitisation at the emitter

Emitters SHALL sanitise `signal.evidence_excerpt` before publishing: ANSI escape sequences stripped, `<` and `>` replaced with safe variants, length capped at 1024 bytes. Larger evidence MUST be referenced via `signal.evidence_ref` (a URI to Loki, S3, etc.) and fetched on demand.

#### Scenario: Oversized log line is truncated and referenced

- **WHEN** a log line exceeds 1024 bytes
- **THEN** the emitter SHALL truncate `signal.evidence_excerpt` at 1024 bytes
- **AND** SHALL set `signal.evidence_ref` to a Loki URL that returns the full line

#### Scenario: ANSI sequences are stripped

- **WHEN** a log line contains terminal colour escapes (`\x1b[31m...\x1b[0m`)
- **THEN** the emitter SHALL emit the excerpt with all escape sequences removed

### Requirement: Emitter back-pressure with local buffer

Each emitter SHALL maintain a bounded local buffer (default 1000 events) when `forge.symptoms.v1` is unavailable. When the buffer is full, the emitter SHALL drop oldest events first and increment `symptom_emitter.dropped_total{emitter}`. Emitters MUST NOT block their source ingest pipeline.

#### Scenario: Bus is briefly unavailable

- **WHEN** Kafka is unreachable for 30 seconds and the emitter generates 200 events in that period
- **THEN** the emitter SHALL buffer the 200 events
- **AND** SHALL flush them in order once the bus is reachable

#### Scenario: Buffer overflow

- **WHEN** the buffer is at capacity (1000) and a new event arrives
- **THEN** the emitter SHALL drop the oldest event and accept the new one
- **AND** SHALL increment `symptom_emitter.dropped_total` by 1
- **AND** SHALL log a structured warning at most once per minute (rate-limited)

### Requirement: Probe emitter has scheduled cadence and contract registry

`symptom-emitter-probe` SHALL maintain a registry of HTTP/TCP/gRPC probes describing what to call, the expected response, and the fingerprint to emit on failure. Probes SHALL run on their configured cadence (default 30s) and emit an event only on **state change** (healthy→unhealthy, unhealthy→healthy with `severity=info`).

#### Scenario: Probe transitions from healthy to unhealthy

- **WHEN** a probe configured for `http://control-plane:8081/healthz` returns non-200 for two consecutive checks (debounce)
- **THEN** the emitter SHALL publish one event with `severity=error` and `signal.fingerprint` including `service:control-plane|signal:probe-failed`
- **AND** SHALL NOT emit again until the probe returns healthy and fails again

#### Scenario: Probe recovers

- **WHEN** a probe transitions from unhealthy to healthy
- **THEN** the emitter SHALL publish one event with `severity=info` and `signal.title="recovery"` so the triager can close any open session for that fingerprint
