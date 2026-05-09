# Spec Delta: policies-and-approvals (MODIFIED)

## MODIFIED Requirements

### Requirement: Workflow publish/install policy templates

The policy engine SHALL provide templates `require-eval-pass`, `require-security-review`, `require-tenant-share-approval`, `forge-certification-prerequisites`.

#### Scenario: Block publish without eval pass

- **GIVEN** policy `require-eval-pass` active for Workspace `ws-1`
- **WHEN** a publish is attempted without a passing eval run
- **THEN** the publish MUST be denied with `eval_pass_missing`

### Requirement: Tenant share approval

`require-tenant-share-approval` template MUST gate promotions to `visibility=tenant` requiring `tenant-admin` approval.

#### Scenario: Tenant promotion approval flow

- **GIVEN** a request to promote a workflow to `tenant` visibility
- **WHEN** policy evaluates
- **THEN** an Approvals Inbox entry MUST be created for `tenant-admin`
- **AND** until approved, visibility MUST remain at the prior tier
