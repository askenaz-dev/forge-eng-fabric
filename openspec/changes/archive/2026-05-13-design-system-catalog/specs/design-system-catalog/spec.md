## ADDED Requirements

### Requirement: Design System catalog with four built-in templates

The platform SHALL publish a Design System catalog of versioned, installable assets. The catalog SHALL ship with four built-in templates at launch: `desing-system-1`, `desing-system-2`, `desing-system-3`, `desing-system-4`. Each template SHALL be published to the platform tenant with `trust_level=T3`, `lifecycle_state=approved`, `visibility=tenant_global`. Each template SHALL include `manifest.tokens` (token sheet URL), `manifest.components` (component pack URL), `manifest.fonts` (preload list), `manifest.screenshots` ({light, dark}), `manifest.use_case` (a short copy block), and an `eval_scores` block covering accessibility (Axe) and brand fidelity.

#### Scenario: Catalog lists the four built-in templates

- **WHEN** any tenant member calls `GET /v1/design-systems`
- **THEN** the response MUST list at least `desing-system-1`, `desing-system-2`, `desing-system-3`, `desing-system-4` with `visibility=tenant_global` and `lifecycle_state=approved`
- **AND** each entry MUST include the screenshots and use-case copy

#### Scenario: Built-in template metadata is complete

- **GIVEN** the built-in template `desing-system-2`
- **WHEN** any caller fetches `GET /v1/design-systems/desing-system-2`
- **THEN** the response MUST include `manifest.tokens`, `manifest.components`, `manifest.fonts`, `manifest.screenshots.light`, `manifest.screenshots.dark`, `manifest.use_case`
- **AND** `eval_scores.accessibility >= 0.9` and `eval_scores.brand_fidelity >= 0.8`

### Requirement: `ds-forge-default` alias

The platform SHALL maintain a permanent alias `ds-forge-default` that resolves to the latest approved version of `desing-system-1`. Downstream tooling (App scaffolder, migration job) SHALL reference the alias rather than a pinned version.

#### Scenario: Alias resolves to current default

- **WHEN** a caller calls `GET /v1/design-systems/ds-forge-default`
- **THEN** the platform MUST resolve the alias to `desing-system-1` at its latest approved version and return the resolved record
- **AND** the response MUST include the header `X-Resolved-From-Alias: ds-forge-default`

#### Scenario: Alias re-target audited

- **WHEN** the platform team re-targets `ds-forge-default` to `desing-system-3@2.0.0`
- **THEN** an `asset.design_system.alias_changed.v1` event MUST be emitted with `before` and `after` blocks and the acting principal

### Requirement: Design System swap on an existing App

The platform SHALL accept `POST /v1/apps/{id}/design-system:swap` with `{target_ref, reason}`. The endpoint MUST (a) verify the caller has `app#owner`, (b) verify the target Design System is `approved` and visible to the App's tenant, (c) open a PR against the App's portal-bundle repository updating `app.config.json` to the new `design_system_ref`, regenerating `tailwind.config.js` token bindings and updating the font preload manifest. The PR SHALL carry the OpenSpec link of the App and the originating user as PR author. The endpoint SHALL emit `app.design_system.swap_requested.v1`.

#### Scenario: Owner swaps the Design System

- **GIVEN** an App `app-1` with `design_system_ref = desing-system-1@1.4.0` and a caller with `app#owner`
- **WHEN** the caller calls `POST /v1/apps/app-1/design-system:swap` with `{target_ref: desing-system-3@2.0.0, reason: "Corporate refresh"}`
- **THEN** the platform MUST verify the target is `approved` and `tenant_global` or shared with the App's tenant
- **AND** open a PR titled "Swap design system to desing-system-3@2.0.0" updating the relevant files
- **AND** emit `app.design_system.swap_requested.v1` with PR URL, target ref and reason
- **AND** return `202 Accepted` with the PR URL

#### Scenario: Swap refused for non-approved target

- **WHEN** an owner attempts to swap to a Design System in `lifecycle_state=proposed`
- **THEN** the platform MUST reject with `409 design_system_not_approved`
- **AND** no PR MUST be opened

#### Scenario: Swap refused without app#owner

- **GIVEN** a caller with `app#editor` but not `app#owner`
- **WHEN** the caller attempts a swap
- **THEN** the platform MUST reject with `403 missing_app_owner`

#### Scenario: Concurrent swap PRs auto-close prior

- **GIVEN** an App with an open swap PR (`pr-1`) opened 10 minutes ago
- **WHEN** an owner opens a second swap PR (`pr-2`) with a different target
- **THEN** the platform MUST close `pr-1` with a comment "Superseded by pr-2"
- **AND** the audit trail MUST link the supersession

### Requirement: Per-component overrides

The platform SHALL accept `PATCH /v1/apps/{id}/design-system/overrides` with a map `{component_name: design_system_ref}` for the components in the canonical primitive list (`button`, `badge`, `card`, `kpi`, `chip`, `seg`, `sheet`, `terminal`, `code`, `run_row`, `approval_card`). Overrides MAY only affect surface tokens (color, typography, border radius, shadow); layout tokens (spacing, grid, breakpoint) MUST come from the App's base Design System. The build-time merger SHALL enforce the namespace whitelist and SHALL reject overrides that would change layout tokens.

#### Scenario: Override card to a different design system

- **GIVEN** an App with `design_system_ref = desing-system-2@1.0.0`
- **WHEN** an owner calls `PATCH /v1/apps/app-1/design-system/overrides` with `{card: desing-system-3@2.0.0}`
- **THEN** the platform MUST persist the override on the App record
- **AND** the next build MUST render `<Card />` using `desing-system-3`'s surface tokens while the rest of the App uses `desing-system-2`
- **AND** the App's spacing scale MUST remain that of `desing-system-2`

#### Scenario: Override targeting layout tokens rejected

- **GIVEN** an override request that would cause a `space-*` token from `desing-system-3` to override the base `desing-system-2`
- **WHEN** the build-time merger evaluates the override
- **THEN** the merger MUST reject the override with `422 layout_token_override_forbidden`
- **AND** the build MUST fail loud with a clear error pointing at the offending token

#### Scenario: Override for unknown component rejected

- **WHEN** an owner attempts to override a component name that is not in the canonical primitive list
- **THEN** the platform MUST reject the request with `422 unknown_component`

### Requirement: Sanity validator for tenant-published Design Systems

A non-built-in Design System asset MUST pass the sanity validator before transitioning to `lifecycle_state=approved`. The validator SHALL reject any token sheet that contains a `url(...)` reference to a domain not on the tenant's approved list, any CSS expression or `expression()` token, any token whose computed value contains a JavaScript scheme, and any custom property name colliding with the layout namespace. Built-in templates SHALL bypass the validator.

#### Scenario: Validator rejects off-tenant URL

- **GIVEN** a Design System asset whose `tokens.css` contains `--logo-bg: url("https://attacker.example/x.png")`
- **WHEN** the publisher requests the transition to `approved`
- **THEN** the validator MUST reject with `422 untrusted_url_in_tokens`
- **AND** the asset MUST stay in `lifecycle_state=in_review`

#### Scenario: Built-in template bypasses validator

- **WHEN** a re-publication of `desing-system-1` is requested
- **THEN** the validator MUST be bypassed and the transition MUST proceed atomically
- **AND** the audit record MUST flag `validator_bypassed=true` with the reason `built_in_template`

### Requirement: CloudEvents for catalog actions

The platform SHALL emit `asset.design_system.published.v1` on initial publication, `asset.design_system.version_published.v1` on new versions, `asset.design_system.alias_changed.v1` on alias re-targeting, `app.design_system.swap_requested.v1` when a swap PR is opened, `app.design_system.changed.v1` when the swap PR merges and the App's `design_system_ref` flips, and `app.design_system.override_changed.v1` when overrides change. Each event SHALL include the App, the Design System ref(s), the principal and a `correlation_id`.

#### Scenario: Swap completion emits change event

- **GIVEN** an open swap PR for `app-1` from `desing-system-1@1.4.0` to `desing-system-3@2.0.0`
- **WHEN** the PR merges and CI completes successfully
- **THEN** the platform MUST update `app-1.design_system_ref=desing-system-3@2.0.0`
- **AND** emit `app.design_system.changed.v1` with `before=desing-system-1@1.4.0`, `after=desing-system-3@2.0.0`, principal and `correlation_id`
