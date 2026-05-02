# Spec Delta: ci-pipeline-baseline (ADDED)

## ADDED Requirements

### Requirement: Mandatory CI stages

Every repo created via Forge MUST run a baseline pipeline with stages: lint, build, unit tests + coverage, secrets scan, SAST, SCA, SBOM, container scan, sign, attest, publish.

#### Scenario: All mandatory stages run on PR

- **GIVEN** a PR opened on a Forge-created repo
- **WHEN** the pipeline runs
- **THEN** all mandatory stages MUST execute and report status checks to GitHub
- **AND** the result of each stage MUST emit `pipeline.gate.evaluated.v1` with stage name, outcome, and severity counts

### Requirement: Configurable thresholds by criticality

Thresholds (coverage minimum, max severity allowed, license allowlist, FP-suppression rules) SHALL be configurable per criticality level via policy.

#### Scenario: Critical app enforces stricter thresholds

- **GIVEN** an app with `criticality=critical`
- **WHEN** SAST reports a `medium` severity finding
- **THEN** the gate MUST fail and block merge
- **AND** emit `pipeline.gate.failed.v1` with severity and finding identifiers

#### Scenario: Low criticality app passes with medium findings

- **GIVEN** an app with `criticality=low`
- **WHEN** SAST reports a `medium` severity finding
- **THEN** the gate MUST pass with warning and emit `pipeline.gate.evaluated.v1` with `outcome=warn`

### Requirement: Override gates with approval

Bypassing a gate SHALL require an approved policy override with TTL ≤ 24h and `security-approver` role; the override MUST be auditable and emitted as `policy.override.granted.v1`.

#### Scenario: Reject merge bypass without approval

- **GIVEN** a failing SAST gate on `criticality=high`
- **WHEN** a merge is attempted without an approved override
- **THEN** the merge MUST be blocked
- **AND** emit `pipeline.gate.bypass.denied.v1`

### Requirement: Gate result persistence

Every gate evaluation MUST be persisted in `pipeline_gate_result` with: PR number, commit SHA, stage, tool, outcome, severity counts, raw report URL, evaluator policy version.

#### Scenario: Auditor retrieves gate history for a PR

- **GIVEN** PR #42 with multiple pipeline runs
- **WHEN** the auditor queries `GET /v1/pipeline-gates?pr=42`
- **THEN** all stage results across all runs MUST be returned with timestamps and policy versions
