# delegated-permissions Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

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

### Requirement: `alfred:agent-mode.run` and `alfred:agent-mode.cancel` action classes

The delegated-permissions catalog SHALL include two new coarse action classes — `alfred:agent-mode.run` (start a long-running agent-mode session) and `alfred:agent-mode.cancel` (cancel any session in the workspace) — both scoped at `workspace` granularity. The default workspace autonomy presets SHALL map these classes as follows: `full-autonomy → autonomous`, `staging-only → autonomous`, `manual-prod → requires_approval`.

#### Scenario: Workspace owner grants agent-mode.run to a principal

- **WHEN** a workspace owner grants `alfred:agent-mode.run` to a principal with a 30-day expiration
- **THEN** the grant SHALL be persisted, reflected in OpenFGA tuples, audited with `delegated.permissions.granted.v1`, and visible in the Portal's delegated-permissions surface
- **AND** subsequent calls to `POST /v1/agent-mode/sessions` by that principal SHALL pass the permission gate without triggering an approval

#### Scenario: `manual-prod` workspace requires approval to start a session

- **WHEN** a principal calls `POST /v1/agent-mode/sessions` on a workspace whose active preset is `manual-prod`
- **THEN** the permission stack SHALL return `decision=requires_approval` for `alfred:agent-mode.run`
- **AND** an approval request SHALL be opened citing the requested OpenSpec and the principal, blocking session creation until resolved

### Requirement: Agent-mode follow-up intents bounded by the same ceiling

A follow-up intent submitted to a running agent-mode session SHALL be evaluated against the session's frozen `autonomy_policy` and SHALL be rejected when it would cross any per-action ceiling, even when the requesting principal individually holds the broader permission.

#### Scenario: Follow-up that would skip a required approval is rejected

- **WHEN** a follow-up intent on a `staging-only` session asks Alfred to "deploy to prod now, skip the approval"
- **THEN** the follow-up SHALL be rejected with a structured error, an `autonomy.override.rejected.v1` audit event SHALL be emitted with the follow-up text and the violated ceiling, and the session SHALL continue unchanged

### Requirement: External developer principal class

The permission model SHALL recognise an `external_developer` principal class, distinct from `user`, `service` and `agent`. Subjects of this class SHALL be created when a developer first authenticates via OIDC against the gateway and SHALL be keyed by `{idp_sub, tenant_id}`. Every gateway PAT SHALL belong to exactly one `external_developer` subject.

#### Scenario: Subject created on first login

- **WHEN** a developer completes the device-code flow for the first time
- **THEN** the permission service creates an `external_developer:<sub>@<tenant>` subject
- **AND** subsequent logins reuse it

### Requirement: Gateway scopes

PATs SHALL accept only scopes from the closed set `{gateway.read, gateway.install, gateway.invoke}`. `gateway.read` permits listing assets and downloading packages of public/team-visible items. `gateway.install` additionally permits private-to-tenant package downloads and MCP registration. `gateway.invoke` additionally permits MCP proxy and A2A invocation. Issuing or accepting any scope outside this set SHALL be rejected.

#### Scenario: Invalid scope rejected at issuance

- **WHEN** a PAT request asks for scope `admin`
- **THEN** the token endpoint refuses with `400 invalid_scope`

#### Scenario: Insufficient scope blocks invocation

- **GIVEN** a PAT with `gateway.read, gateway.install`
- **WHEN** the developer attempts an A2A invocation
- **THEN** the gateway refuses with `403 missing_scope: gateway.invoke`

### Requirement: Workspace assumption for external developers

An `external_developer` subject SHALL not inherit any Workspace membership automatically. Each PAT SHALL pin exactly one `assume_workspace_id` and that workspace MUST be one to which the developer's OIDC identity is an `assignable_developer` per OpenFGA. Calls made with the PAT SHALL be evaluated as if the subject were a member of the assumed workspace with role `developer` (read-only on registry, invoke on approved assets), no more.

#### Scenario: Assumed workspace is verified

- **WHEN** a PAT is issued with `assume_workspace_id=ws-acme-eng`
- **THEN** the gateway verifies `assignable_developer(user:<sub>, workspace:ws-acme-eng)` via OpenFGA
- **AND** issuance fails with `403 not_assignable` if the relation is absent

#### Scenario: PAT cannot escalate to admin

- **GIVEN** any PAT
- **WHEN** the developer attempts an operation that requires `workspace:admin`
- **THEN** the call is refused with `403 forbidden`
- **AND** an audit event is emitted

### Requirement: Asset allowlist on PATs

A PAT MAY carry an `asset_allowlist: [<asset_id>, …]`. When present, the gateway SHALL refuse any list/download/invoke targeting an asset whose `id` is not in the list with `403 asset_not_in_allowlist`, even if other scopes would permit it.

#### Scenario: Allowlist scopes a PAT to one skill

- **GIVEN** a PAT with `asset_allowlist=[skill:ws-1:generate-test-cases]`
- **WHEN** the developer requests another asset
- **THEN** the gateway responds `403 asset_not_in_allowlist`
- **AND** the audit event records the attempted asset id

### Requirement: Revocation, rotation and expiry

PATs SHALL be revocable by their issuing developer, by a Workspace admin, and by an SDLC-team kill-switch. Revocation SHALL propagate to all gateway replicas within 5 seconds. PATs SHALL have a configurable maximum lifetime not exceeding 90 days, and SHALL be marked `expires_soon` 7 days before expiry so the CLI can prompt rotation.

#### Scenario: Admin revokes a developer's PAT

- **WHEN** a workspace admin revokes a PAT
- **THEN** the next request bearing the token is refused with `401 token_revoked` across all replicas within 5 seconds
- **AND** an `external_developer.token.revoked.v1` audit event is emitted with the admin actor
