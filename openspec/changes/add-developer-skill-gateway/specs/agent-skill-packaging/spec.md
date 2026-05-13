## ADDED Requirements

### Requirement: Open Agent Skills layout

The packager SHALL produce bundles in the open Agent Skills format: a top-level folder named after the asset's `name` containing a required `SKILL.md` file and optional `scripts/`, `references/`, `assets/` and free-form subdirectories. `SKILL.md` SHALL begin with YAML front-matter containing at least `name` and `description` and SHALL follow with markdown instructions.

#### Scenario: Bundle is a single folder

- **WHEN** the packager packages an approved skill named `generate-test-cases@1.2.0`
- **THEN** the bundle contains exactly one top-level directory `generate-test-cases/`
- **AND** that directory contains `SKILL.md`

#### Scenario: SKILL.md front-matter is well-formed

- **WHEN** any consumer parses the produced `SKILL.md`
- **THEN** the YAML front-matter exposes `name` matching the registry name and `description` matching the registry description

### Requirement: Deterministic, content-addressed bundles

For a given `(asset_id, version)`, the packager SHALL produce a byte-identical `.tar.zst` bundle on every run. The sha256 digest of the bundle SHALL be persisted in the registry as `asset_package.digest` and SHALL be the canonical identifier used by the gateway, CLI and audit pipeline.

#### Scenario: Re-packaging yields the same digest

- **WHEN** the packager is invoked twice for the same `(asset_id, version)` against an unchanged source
- **THEN** the resulting bundles have identical sha256 digests
- **AND** mtime, ownership, ordering and compression parameters are normalized to fixed values

#### Scenario: Source change forces a new asset version

- **GIVEN** an existing approved version `1.2.0`
- **WHEN** the source changes
- **THEN** the packager refuses to publish under `1.2.0`
- **AND** instructs the publisher to bump SemVer

### Requirement: Provenance and signature

Every produced bundle SHALL be signed with the platform cosign identity and accompanied by an in-toto attestation referencing the source commit, the source repo URL, the publishing pipeline run ID and the registry `asset_id@version`. The registry SHALL store both signature and attestation alongside the digest.

#### Scenario: Signature verifies against the platform key

- **WHEN** the CLI verifies a downloaded bundle
- **THEN** cosign verification succeeds against the published Forge signing key
- **AND** the attestation references the commit currently approved in the registry

#### Scenario: Missing attestation blocks publication

- **WHEN** the packager runs without an attestation input
- **THEN** publication MUST fail with `412 attestation_required`

### Requirement: Reference-material expansion

When a skill declares `references` in the registry (URLs or repo paths), the packager SHALL fetch and embed them under `references/` in the bundle so the skill remains usable offline. Each embedded file SHALL keep its source URL and sha256 recorded in `references/INDEX.json`.

#### Scenario: External reference is embedded

- **GIVEN** a skill declaring `references: ["https://docs.example/spec.md"]`
- **WHEN** the bundle is built
- **THEN** the file is present at `references/spec.md`
- **AND** `references/INDEX.json` contains `{"path":"references/spec.md","url":"https://docs.example/spec.md","sha256":"…"}`

#### Scenario: Reference fetch failure is hard-fail

- **WHEN** any referenced URL returns non-2xx
- **THEN** packaging fails with `502 reference_fetch_failed` and no partial bundle is published

### Requirement: Eligible asset types

The packager SHALL accept asset types `skill` and `agent` (light agents with no runtime state). Asset types `mcp`, `workflow` and `prompt_template` SHALL NOT be packaged as Agent Skills — they are exposed by the gateway through MCP proxy, A2A endpoint or the internal marketplace respectively.

#### Scenario: MCP cannot be packaged

- **WHEN** the packager is asked to package an MCP asset
- **THEN** it refuses with `400 not_packageable`
- **AND** the error suggests using the MCP proxy endpoint

### Requirement: Bundle size and safety limits

The packaged bundle SHALL be at most 50 MB compressed and 250 MB uncompressed; SHALL contain no symlinks pointing outside the bundle root, no setuid bits and no character / block devices; and SHALL refuse to embed files matching the platform's secret-detection patterns (private keys, `.env`, credential JSONs).

#### Scenario: Secret-shaped file is refused

- **GIVEN** a source containing `secrets.env`
- **WHEN** the packager runs
- **THEN** it fails with `400 secret_material_in_source`
- **AND** the failure names the offending path
