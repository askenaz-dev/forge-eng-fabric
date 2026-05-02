## ADDED Requirements

### Requirement: Explicit, scoped, auditable, revocable grants
Alfred's elevated permissions SHALL be granted explicitly by an authorized owner, scoped to a target (Workspace/Repo/Environment/Cloud Project) and an `action_class`, with a `max_criticality` and an `expiration`. Grants SHALL be auditable and revocable at any time.

#### Scenario: Owner grants Alfred a scoped permission
- **WHEN** a Workspace owner grants Alfred a scoped permission with criticality and expiration
- **THEN** the grant is persisted, reflected in OpenFGA tuples, audited, and visible in the Portal

#### Scenario: Owner revokes a grant
- **WHEN** a Workspace owner revokes a grant
- **THEN** Alfred can no longer perform the corresponding actions on that scope, the revocation is audited and propagated to OpenFGA immediately

### Requirement: Federated cloud project support
The platform SHALL support delegating permissions to Alfred over **federated cloud projects** (target projects belonging to teams/initiatives), preserving explicit scope and audit.

#### Scenario: Federated grant is auditable end-to-end
- **WHEN** a target-project owner grants Alfred a scoped role on a federated GCP project
- **THEN** the grant is recorded with project, scope, action_class, expiration, requester, approver, and is queryable in audit

### Requirement: Default expirations and review
Grants SHALL have a default expiration (configurable, conservative by default) and SHALL be reviewable periodically by the SDLC Team. Re-granting SHALL be an explicit operation.

#### Scenario: Grant expires automatically
- **WHEN** a grant reaches its expiration
- **THEN** Alfred loses the corresponding permission, owners are notified, and the expiration is audited

### Requirement: UI for granting/revoking permissions
The Portal SHALL provide a UI to grant, view, audit and revoke Alfred's delegated permissions with full context (scope, action_class, criticality, expiration, justification, requester, approver).

#### Scenario: Owner inspects and revokes a grant from the Portal
- **WHEN** an owner reviews active grants in the Portal
- **THEN** the owner can revoke any grant immediately and the revocation is audited
