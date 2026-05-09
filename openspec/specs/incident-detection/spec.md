# incident-detection Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Multi-source detection

The service MUST normalize signals from Prometheus alertmanager, Cloud Monitoring, Loki alert rules, and internal CloudEvents (`slo.burn-rate.fast.v1`, `cost.spike.v1`, `eval.regression.detected.v1`, `iac.drift.detected.v1`, `deployment.failed.v1`) into `incident.detected.v1`.

#### Scenario: Prometheus alert normalized

- **GIVEN** a Prometheus alert webhook payload `HighErrorRate{service=app-foo, env=prod}`
- **WHEN** the detection service receives it
- **THEN** it MUST emit `incident.detected.v1` with `service=app-foo, env=prod, source=prometheus, signature_hash=...`

### Requirement: Deduplication and correlation

Identical incidents within a 5-minute window (matched by `service+env+signature_hash`) MUST be deduplicated into a single open incident; subsequent occurrences MUST update the existing record.

#### Scenario: Dedup within window

- **GIVEN** an open incident `inc-1` from 2 minutes ago
- **WHEN** an identical alert fires
- **THEN** the service MUST attach the new event to `inc-1`
- **AND** MUST NOT emit a new `incident.detected.v1`

### Requirement: Manual declaration

A human MAY declare an incident via `POST /v1/incidents/declare`; the service MUST treat manual declarations equivalently for downstream consumers.

#### Scenario: Manual declare creates incident

- **GIVEN** an SRE noticing a problem absent from automated alerts
- **WHEN** they call `POST /v1/incidents/declare`
- **THEN** an incident MUST be created with `source=manual`
- **AND** emit `incident.detected.v1` with the declarer as actor
