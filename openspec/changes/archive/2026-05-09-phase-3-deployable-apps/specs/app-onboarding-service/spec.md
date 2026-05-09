# Spec Delta: app-onboarding-service (MODIFIED)

## MODIFIED Requirements

### Requirement: Optional runtime defaults at onboarding

The onboarding service SHALL support optional runtime defaults and, when requested, MUST provision a default runtime (typically `dev`) by invoking `POST /v1/runtimes/provision` (Provisioned mode) or accept a pre-existing BYO runtime id.

#### Scenario: Onboarding with provisioned dev runtime

- **GIVEN** a Workspace requesting onboarding with `provision_dev_runtime=true`
- **WHEN** onboarding completes the repo creation phase
- **THEN** the service MUST trigger runtime provisioning for `env=dev`
- **AND** wait for `runtime.provisioned.v1`
- **AND** emit `app.onboarding.runtime_provisioned.v1`

#### Scenario: Onboarding with BYO runtime reference

- **GIVEN** a Workspace passing `byo_runtime_id=rt-7`
- **WHEN** onboarding processes runtime defaults
- **THEN** the service MUST validate that `rt-7` belongs to the same Workspace
- **AND** record the linkage on the application asset
- **AND** SHALL NOT provision new infrastructure

### Requirement: Reject onboarding referencing inaccessible runtime

If a referenced runtime is from another Workspace or is revoked, onboarding MUST fail at the runtime-defaults stage.

#### Scenario: Cross-Workspace runtime reference rejected

- **GIVEN** an onboarding in `ws-1` referencing a runtime in `ws-2`
- **WHEN** runtime defaults are evaluated
- **THEN** onboarding MUST fail with `403 cross_workspace_runtime`
- **AND** the repo creation MUST NOT be reverted (already committed) but the application asset MUST stay in `lifecycle_state=proposed` with annotation
