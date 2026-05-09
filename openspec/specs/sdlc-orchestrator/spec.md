# sdlc-orchestrator Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Initiative model with phase state machine

The orchestrator SHALL maintain initiatives backed by a phase state machine over `product → architecture → design → development → qa → security → devops → sre → finops`; transitions MUST be persisted with timestamps and actors.

#### Scenario: Initiative created from OpenSpec root

- **GIVEN** an OpenSpec `spec-7` in `lifecycle_state=approved`
- **WHEN** `POST /v1/initiatives` is called with `openspec_root=spec-7`
- **THEN** an initiative MUST be created with `current_phase=product, status=in_progress`
- **AND** emit `sdlc.phase.entered.v1{phase=product}`

### Requirement: Gate evaluation between phases

Phase progression MUST require all gates of the current phase to pass; gate evaluation MUST emit events with detail per gate.

#### Scenario: Architecture gate fails on missing ADRs

- **GIVEN** an initiative in `phase=architecture` with no ADRs published
- **WHEN** the orchestrator evaluates gates
- **THEN** gate `adrs_published` MUST be `failed`
- **AND** emit `sdlc.phase.gate_evaluated.v1{phase=architecture, gate=adrs_published, outcome=failed}`
- **AND** the initiative MUST stay in `phase=architecture, status=blocked`
- **AND** create a blocker entry visible in the Approvals Inbox

#### Scenario: Successful progression

- **GIVEN** an initiative whose all `phase=design` gates pass
- **WHEN** the orchestrator evaluates progression
- **THEN** the initiative MUST move to `phase=development`
- **AND** emit `sdlc.phase.progressed.v1{from=design, to=development}`

### Requirement: Override with approval

A `phase-progression-bypass` override MAY unblock progression with TTL ≤ 24h and `release-manager` approval; the bypass MUST record explicit rationale.

#### Scenario: Bypass approval consumes once

- **GIVEN** an approved bypass for `phase=qa` of `initiative-3` with TTL=4h and rationale "hotfix release"
- **WHEN** the orchestrator processes progression
- **THEN** progression MUST proceed despite failing gate
- **AND** emit `policy.override.consumed.v1` and `sdlc.phase.progressed.v1{override=true}`
- **AND** the override MUST NOT apply to future transitions

### Requirement: Idempotent transitions

Repeated transition triggers MUST be idempotent: a duplicate completion call returns the current state without side effects.

#### Scenario: Duplicate complete-phase call

- **GIVEN** an initiative already progressed past `phase=design`
- **WHEN** a stale `complete` call for `phase=design` arrives
- **THEN** the orchestrator MUST return current state with HTTP 200
- **AND** emit no additional events
