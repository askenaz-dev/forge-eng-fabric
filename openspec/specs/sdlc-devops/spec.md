# sdlc-devops Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: DevOps skills

The capability SHALL expose `prepare-release-notes`, `validate-rollback-plan`, `update-pipeline` as registered skills.

#### Scenario: Release notes generated from changes

- **GIVEN** an initiative ready to deploy with linked PRs
- **WHEN** Alfred invokes `prepare-release-notes`
- **THEN** notes MUST be produced grouping changes by type (feature/fix/chore)
- **AND** linking each entry to PR + OpenSpec
- **AND** stored in the release artifact

### Requirement: DevOps gates

Gates `pipelines_green`, `image_signed`, `deploy_to_stage_successful`, `rollback_plan_present` MUST be evaluated before progression to `sre`.

#### Scenario: Block progression when stage deploy failed

- **GIVEN** an initiative whose latest stage deploy ended in `deployment.failed.v1`
- **WHEN** progression is requested
- **THEN** gate `deploy_to_stage_successful` MUST fail
- **AND** Alfred MUST surface the failure reason and propose retry
