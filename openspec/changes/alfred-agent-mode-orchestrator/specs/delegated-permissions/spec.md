## ADDED Requirements

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
