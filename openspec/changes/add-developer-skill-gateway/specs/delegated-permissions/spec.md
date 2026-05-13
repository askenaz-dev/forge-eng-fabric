## ADDED Requirements

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
