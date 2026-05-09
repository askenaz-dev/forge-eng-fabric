# pr-openspec-linking Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: PR description must reference OpenSpec

Every PR on a Forge-managed repo MUST include at least one `OpenSpec: <id>` reference in its description for `criticality≥medium`; for `criticality=low` it is recommended but not required.

#### Scenario: Block merge for medium criticality without link

- **GIVEN** a PR on a `criticality=medium` repo with no `OpenSpec:` reference
- **WHEN** the check `forge/openspec-link` runs
- **THEN** the check MUST fail with reason `missing_openspec_link`
- **AND** the merge MUST be blocked
- **AND** emit `pr.openspec_link.missing.v1`

#### Scenario: Allow merge for low criticality without link

- **GIVEN** a PR on a `criticality=low` repo with no link
- **WHEN** the check runs
- **THEN** the check MUST pass with status `neutral`
- **AND** emit `pr.openspec_link.warning.v1`

### Requirement: OpenSpec validation

The check MUST validate that referenced OpenSpec ids exist, belong to the same Workspace as the repo, and are in `lifecycle_state ∈ {approved, in_review}`.

#### Scenario: Reject PR linked to retired OpenSpec

- **GIVEN** a PR referencing `OpenSpec: spec-42` where `spec-42.lifecycle_state=retired`
- **WHEN** the check runs
- **THEN** the check MUST fail with reason `openspec_not_active`

#### Scenario: Reject PR linked to OpenSpec from another Workspace

- **GIVEN** a PR in `ws-1` referencing an OpenSpec belonging to `ws-2`
- **WHEN** the check runs
- **THEN** the check MUST fail with reason `openspec_workspace_mismatch`

### Requirement: Bidirectional traceability on merge

When a PR is merged, the OpenSpec MUST receive a `decision_log` entry containing: PR URL, merge SHA, author, merged_at, and the corresponding `pr.linked-to-openspec.v1` event MUST be emitted.

#### Scenario: OpenSpec decision log updated on merge

- **GIVEN** PR #42 referencing `spec-7` is merged with SHA `abc123`
- **WHEN** the merge webhook is processed
- **THEN** `spec-7.decision_log` MUST include an entry with `pr_url`, `sha=abc123`, `author`, `merged_at`
- **AND** `pr.linked-to-openspec.v1` MUST be emitted

### Requirement: Override with approval

Merging without an OpenSpec link on `criticality≥medium` SHALL require an approved `merge-without-openspec` policy override, fully audited.

#### Scenario: Approved override unblocks merge

- **GIVEN** a PR on `criticality=high` without OpenSpec link
- **AND** an approved override `merge-without-openspec` with TTL=2h
- **WHEN** the merge is attempted within TTL
- **THEN** the merge MUST proceed
- **AND** emit `policy.override.consumed.v1` with override id, PR id, and approver
