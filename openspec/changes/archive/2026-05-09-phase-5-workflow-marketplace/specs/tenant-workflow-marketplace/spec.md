# Spec Delta: tenant-workflow-marketplace (ADDED)

## ADDED Requirements

### Requirement: Visibility tiers

The marketplace SHALL support visibility tiers `private`, `workspace`, `tenant`, `forge-certified`; promotion across tiers MUST require approval.

#### Scenario: Promotion to tenant requires Tenant admin approval

- **GIVEN** a workflow `wf-1@1.0.0` with `visibility=workspace`
- **WHEN** the owner requests promotion to `tenant`
- **THEN** an approval entry MUST be created for `tenant-admin`
- **AND** until approved, the workflow MUST NOT appear in the Tenant catalog

#### Scenario: Forge-certified requires eval pass + security review

- **GIVEN** a workflow seeking `forge-certified`
- **WHEN** certification is requested
- **THEN** the registry MUST verify `eval_run.outcome=passed` and a recorded `security-review`
- **AND** lacking either, refuse with `412 certification_prerequisites_missing`

### Requirement: Install pins exact version

Installation to a Workspace MUST pin to an exact `workflow_id@version` and create a `workflow_install` record.

#### Scenario: Install record persisted

- **GIVEN** a user installs `wf-1@1.2.0` to `ws-1`
- **WHEN** the install completes
- **THEN** a `workflow_install` MUST exist with `workspace=ws-1, workflow=wf-1, version=1.2.0`
- **AND** emit `workflow.installed_to_workspace.v1`

### Requirement: Catalog search

`GET /v1/marketplace` MUST support filters by `visibility`, `tags`, `criticality`, `text-search`; results MUST respect Tenant boundaries.

#### Scenario: User in Tenant A cannot see Tenant B private workflows

- **GIVEN** a workflow private to `tenantB`
- **WHEN** a user in `tenantA` queries the marketplace
- **THEN** the workflow MUST NOT appear in results
