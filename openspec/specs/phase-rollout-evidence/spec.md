# phase-rollout-evidence Specification

## Purpose
TBD - created by archiving change platform-gaps-closure. Update Purpose after archive.
## Requirements

### Requirement: Sign-off evidence file per phase

The platform SHALL maintain a sign-off evidence file at `docs/governance/phase-<n>-signoff.md` for every roadmap phase from Phase 0 through Phase 6. Each file SHALL exist before the phase is considered complete.

#### Scenario: Sign-off file exists for each completed phase

- **WHEN** a release describes a phase as complete in `README.md` or `docs/platform-enablement.md`
- **THEN** the corresponding `docs/governance/phase-<n>-signoff.md` SHALL exist and SHALL be referenced from the release notes

### Requirement: Required sections in sign-off file

Each sign-off file SHALL contain the following sections: scope summary, exit criteria checklist with each item linked to the spec or task in the archive, evidence links (PRs, runbook executions, eval reports, demo recordings), known deferred items, and named approvers with role and date.

#### Scenario: Exit criteria checklist links to archive

- **WHEN** a reviewer audits a sign-off file
- **THEN** every exit-criteria item SHALL link to the corresponding archived OpenSpec change in `openspec/changes/archive/` or to the corresponding spec in `openspec/specs/`

#### Scenario: Approvers named and dated

- **WHEN** a sign-off file is finalized
- **THEN** approvers SHALL be listed with name, role, and signed date (ISO-8601), and the file SHALL match the approver set required by the phase's approval policy

### Requirement: Signed git tag accompanies sign-off

When a sign-off file is finalized, a signed git tag named `phase-<n>-signoff-<YYYYMMDD>` SHALL be created at the commit that finalizes the file, providing a tamper-evident anchor.

#### Scenario: Tag created on sign-off

- **WHEN** the merge that finalizes the sign-off lands on the default branch
- **THEN** a release engineer SHALL create the signed tag pointing at that commit, and the tag SHALL be visible via `git tag --list 'phase-*-signoff-*'`

### Requirement: Deferred items tracked as follow-up changes

Deferred items recorded in a sign-off file SHALL each map to an open OpenSpec change or an issue with a clear owner and target date.

#### Scenario: Each deferred item has an owner and follow-up

- **WHEN** a sign-off lists an item as deferred
- **THEN** the item SHALL include a link to the follow-up OpenSpec change or issue, and SHALL include an owner and target completion date

### Requirement: Phase 0 and Phase 1 sign-offs produced

The platform SHALL produce sign-off evidence files for Phase 0 (`phase-0-foundations`) and Phase 1 (`phase-1-agentic-core`), closing the open tasks listed in their archived `tasks.md` and recording any items that remain deferred.

#### Scenario: Phase 0 sign-off published

- **WHEN** Phase 0 sign-off is complete
- **THEN** `docs/governance/phase-0-signoff.md` SHALL exist, list approvers, link to evidence including the cloud bootstrap evidence, GitHub App registration evidence, and the Langfuse staging evidence (or list each as deferred with a follow-up)

#### Scenario: Phase 1 sign-off published

- **WHEN** Phase 1 sign-off is complete
- **THEN** `docs/governance/phase-1-signoff.md` SHALL exist with approvers, evidence of integrated staging, SDLC orchestrator end-to-end execution, and any deferred items linked to follow-up changes

### Requirement: Phase 2–6 sign-offs and enablement sections

For each of Phases 2 through 6, the platform SHALL complete the corresponding section in `docs/platform-enablement.md`, the runbook(s) referenced from that section, and a sign-off file `docs/governance/phase-<n>-signoff.md`. The enablement section SHALL be operationally sufficient: an experienced operator can execute the phase from the documentation alone.

#### Scenario: Phase enablement section is operationally sufficient

- **WHEN** an operator who has not previously rolled out the phase follows only `docs/platform-enablement.md` and the linked runbooks
- **THEN** they SHALL successfully complete the phase rollout, or they SHALL identify a documentation defect that must be fixed before sign-off

#### Scenario: Phase 2 sign-off references productive artifacts

- **WHEN** Phase 2 is signed off
- **THEN** the sign-off file SHALL reference: a registered GitHub App, a reusable CI workflow used by at least one repository scaffolded by the platform, SBOM/Cosign/Trivy evidence for at least one image, and an Artifact Registry record

#### Scenario: Phase 3 sign-off references runtime onboarding

- **WHEN** Phase 3 is signed off
- **THEN** the sign-off file SHALL reference: at least one BYO runtime onboarded, at least one Provisioned-by-Forge runtime onboarded, a successful `make verify-runtime` report for each, and image-verification-at-deploy evidence

#### Scenario: Phase 4 sign-off references SDLC

- **WHEN** Phase 4 is signed off
- **THEN** the sign-off file SHALL reference: registered Skills with eval reports per capability, policies bound to capabilities, prompt templates seeded in the registry, and a successful run of the reference workflow

#### Scenario: Phase 5 sign-off references workflow runtime and marketplace

- **WHEN** Phase 5 is signed off
- **THEN** the sign-off file SHALL reference: a durable workflow runtime running at least one long-lived workflow, a functioning internal marketplace listing reusable workflows, and an advanced eval-harness run

#### Scenario: Phase 6 sign-off references autonomous ops

- **WHEN** Phase 6 is signed off
- **THEN** the sign-off file SHALL reference: the healing actions catalog, at least one simulated remediation under guardrails, and an evolution-loop record updating an OpenSpec
