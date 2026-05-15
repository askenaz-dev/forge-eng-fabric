## ADDED Requirements

### Requirement: Infrastructure skills

The capability SHALL expose `generate-terraform`, `generate-helm-values`, `validate-iac`, `apply-iac` as registered skills in the Asset Registry, owned by the platform infrastructure team. Each skill SHALL ship with an eval suite covering at minimum 30 graded fixtures and a documented promotion threshold per trust level.

#### Scenario: generate-terraform produces a module for the App's stack

- **GIVEN** an App `app-1` with `targets.iac in {required, optional, opt-in}` and a resolved architecture decision recording (e.g., "Python service on managed Postgres on GCP")
- **WHEN** Alfred invokes `generate-terraform` against `app-1`
- **THEN** the skill MUST produce a Terraform module under `infra/<app-slug>/terraform/` containing `main.tf`, `variables.tf`, `outputs.tf`, `versions.tf` and a per-environment overlay directory (`environments/{local,staging,prod}`)
- **AND** emit `sdlc.iac.generated.v1` with the module path, the provider list and the OpenSpec link

#### Scenario: generate-helm-values renders values per criticality tier

- **WHEN** Alfred invokes `generate-helm-values` against `app-1` with `criticality=medium`
- **THEN** the skill MUST produce `infra/<app-slug>/helm/values-{local,staging,prod}.yaml` whose replica counts, requests and limits MUST match the medium-tier values from the platform sizing document
- **AND** the resulting values MUST validate against the App's chart `Chart.yaml`

### Requirement: validate-iac runs plan + lint + conftest

The `validate-iac` skill SHALL run, in this order, against any generated IaC bundle: `terraform fmt` (must be clean), `terraform plan` against the target environment (must succeed without errors), `helm lint` (must be clean), `helm template` (must render successfully), `conftest test` against the platform's policy bundle (must produce zero violations). The skill SHALL produce a structured `iac_validation_report` and emit `sdlc.iac.validated.v1` with the report.

#### Scenario: Validation gate fails on conftest violation

- **GIVEN** a generated Helm bundle that omits `NetworkPolicy`
- **WHEN** `validate-iac` runs
- **THEN** the conftest step MUST report a violation `missing_network_policy`
- **AND** the skill MUST return `iac_validation_report.status=failed`
- **AND** the workflow gate `iac_validated` MUST fail with reason `policy_violation:missing_network_policy`

#### Scenario: Validation gate passes on a clean bundle

- **WHEN** all four sub-checks pass
- **THEN** the report MUST be `status=passed` and the gate `iac_validated` MUST allow progression

### Requirement: apply-iac is PR-driven; no direct apply from the skill

The `apply-iac` skill SHALL NEVER execute `terraform apply` or `helm upgrade` directly. Instead, the skill SHALL open a PR against the App's infra repository (`infra/<app-slug>/`) containing: the generated `.tf` and Helm files, a checked-in `terraform plan` output as a PR comment, the `helm template` output, the `iac_validation_report` from `validate-iac`. Merging the PR SHALL trigger the platform's GitOps runner which performs the actual `terraform apply` and `helm upgrade --install`. The skill SHALL emit `sdlc.iac.applied.v1` with the PR URL and the resulting deployment refs after the GitOps runner completes.

#### Scenario: Apply produces a PR not a direct apply

- **GIVEN** a clean IaC bundle for `app-1` and an authorized invocation of `apply-iac`
- **WHEN** the skill runs
- **THEN** the skill MUST open a PR titled `infra: apply <app-slug> <env>` against the App's infra repo
- **AND** the PR description MUST include the `iac_validation_report`, the `terraform plan` output and the `helm template` output
- **AND** the skill MUST NOT call `terraform apply` directly
- **AND** an `sdlc.iac.applied.v1` event MUST be emitted only after the GitOps runner reports success on the merged PR

#### Scenario: Break-glass apply with elevated approval

- **GIVEN** an emergency hotfix where the operator passes `break_glass=true` and has the joint approval of `security-admin` and `platform-admin`
- **WHEN** `apply-iac` runs with `break_glass=true`
- **THEN** the skill MAY trigger an out-of-PR apply through the GitOps runner with both approver identities recorded
- **AND** an audit event `sdlc.iac.break_glass_applied.v1` MUST be emitted with both principals, the OpenSpec link, the diff and the reason

### Requirement: Infrastructure gates

Gates `iac_generated`, `iac_validated`, `iac_applied` SHALL be evaluated in order before progression to `sre`. The reference workflow SHALL only run these gates when `App.targets.iac != skipped`.

#### Scenario: Skipped iac removes the gates from the plan

- **GIVEN** an App with `targets.iac=skipped`
- **WHEN** the workflow plan is built
- **THEN** the plan MUST NOT include `iac_generated`, `iac_validated` or `iac_applied`
- **AND** progression to `sre` MUST proceed without evaluating these gates

#### Scenario: Required iac blocks progression on validation failure

- **GIVEN** an App with `targets.iac=required` and a failed `validate-iac` run
- **WHEN** progression is requested
- **THEN** the gate `iac_validated` MUST fail
- **AND** `sdlc.phase.blocked.v1` MUST be emitted with `phase=iac, reason=validation_failed`

### Requirement: Provider matrix matches platform runtimes

The Terraform modules and Helm value sets SHALL cover the three cloud providers the platform supports for runtimes (AWS, GCP, Azure). The skill SHALL refuse to generate IaC for a provider not on this list.

#### Scenario: Unsupported provider rejected

- **WHEN** an App is configured for a runtime targeting an unsupported provider (e.g., on-prem-only proprietary)
- **THEN** `generate-terraform` MUST refuse with `422 unsupported_provider` listing the supported set
