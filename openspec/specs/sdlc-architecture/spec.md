# sdlc-architecture Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements
### Requirement: Architecture skills

The capability SHALL expose `propose-adr`, `evaluate-options`, `check-openspec-alignment` as registered skills with eval suites.

#### Scenario: ADR generated and stored

- **GIVEN** an initiative in `phase=architecture` with multiple options
- **WHEN** Alfred invokes `propose-adr`
- **THEN** an ADR MUST be created (in repo `docs/adr/` or Confluence) with status `proposed`
- **AND** linked to the OpenSpec via `decision_log`
- **AND** emit `sdlc.adr.proposed.v1`

### Requirement: Architecture gates

Gates `adrs_published`, `security_review_passed`, `openspec_updated`, **`data_model_documented`** (for Apps with `targets.architecture != skipped` and `criticality≥medium`), **`api_contract_published`** (for Apps with `targets.architecture != skipped`) MUST be evaluated before progression to `design`.

#### Scenario: Block on missing security review for high criticality

- **GIVEN** an initiative with `criticality=high`
- **WHEN** the architecture phase has ADRs but no recorded security review
- **THEN** gate `security_review_passed` MUST fail with reason `security_review_missing`
- **AND** emit `sdlc.phase.blocked.v1`

#### Scenario: Block on missing API contract when targets demand it

- **GIVEN** an App with `targets.architecture=required` and no published API contract
- **WHEN** progression to `design` is requested
- **THEN** gate `api_contract_published` MUST fail
- **AND** Alfred MUST surface the missing artefact and propose invoking `generate-api-contract`

### Requirement: Architecture skill implementations

The capability SHALL ship working implementations of `propose-adr`, `evaluate-options`, `check-openspec-alignment` skills, registered in the Asset Registry with their eval suites. The skills SHALL be invokable through the standard skill-gateway and SHALL persist their outputs in the App's repo and in the OpenSpec decision log.

#### Scenario: propose-adr writes to docs/adr and to the decision log

- **GIVEN** an App `app-1` in `phase=architecture` with multiple options surfaced by `evaluate-options`
- **WHEN** Alfred invokes `propose-adr`
- **THEN** the skill MUST produce an ADR file at `docs/adr/<NNNN>-<slug>.md` in the App's repo following the MADR template
- **AND** append an entry to the OpenSpec `decision_log` with `{type: adr, key: "ADR-<NNNN>", url, created_by: alfred, at}`
- **AND** emit `sdlc.adr.proposed.v1` with the ADR path and the OpenSpec link

#### Scenario: evaluate-options ranks alternatives with cited rationale

- **WHEN** Alfred invokes `evaluate-options` with at least 2 candidate options
- **THEN** the skill MUST produce a ranked list with `pros[], cons[], score, rationale` per option
- **AND** every rationale MUST cite at least one source (RAG-retrieved doc, ADR, OpenSpec) — uncited rationales MUST be discarded by the citation enforcer

#### Scenario: check-openspec-alignment blocks on missing requirement coverage

- **GIVEN** a proposed ADR and an OpenSpec with declared requirements
- **WHEN** `check-openspec-alignment` runs
- **THEN** the skill MUST report `aligned=false` if any OpenSpec requirement is unaddressed by the architecture
- **AND** the gate `openspec_updated` MUST fail with the unaddressed requirement list

### Requirement: API contract / data model / threat model implementations

The capability SHALL ship working implementations of `generate-api-contract`, `propose-data-model`, `lightweight-threat-model` (the existing scenarios in `sdlc-design` cover the API-contract flow; this requirement covers the architecture-side wiring for data model and threat model).

#### Scenario: propose-data-model produces a normalised ER schema

- **GIVEN** an App `app-1` with an approved API contract and a defined data sensitivity classification
- **WHEN** Alfred invokes `propose-data-model`
- **THEN** the skill MUST produce a data model file at `docs/data-model/<spec-slug>.md` with entities, relationships, primary keys, foreign keys and per-field sensitivity tagging
- **AND** emit `sdlc.data_model.proposed.v1` with the file path and the OpenSpec link

#### Scenario: lightweight-threat-model required at criticality medium and above

- **GIVEN** an App with `criticality in {medium, high, critical}`
- **WHEN** progression past the architecture phase is requested without a threat model
- **THEN** the gate `threat_model_present` MUST fail
- **AND** Alfred MUST automatically invoke `lightweight-threat-model` when `targets.security in {required, autonomous}` and persist the result in `docs/threat-models/<spec-slug>.md`
- **AND** emit `sdlc.threat_model.completed.v1` with the file path

