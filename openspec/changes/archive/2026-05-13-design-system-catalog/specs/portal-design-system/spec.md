## MODIFIED Requirements

### Requirement: Design-system stylesheet is the sole source of branded styles

For each Portal-rendered App, the resolved Design System (the asset referenced by `App.design_system_ref`) SHALL be the sole source of branded styles. The token sheet SHALL be inlined at build time from the asset's `manifest.tokens`. No other CSS file inside the App's portal bundle SHALL declare colour, font or shadow values; component-specific layout-only CSS modules are permitted if they consume only design-system tokens. The legacy classes used in the existing Tailwind shell (`bg-neutral-50`, `bg-white`, `dark:bg-neutral-900`, ad-hoc `rounded`, `border-neutral-*`) SHALL be removed. The Portal's own admin surfaces SHALL continue to use `desing-system-1` (the package wrapping the current default Portal design system) via `ds-forge-default`.

#### Scenario: Legacy neutral colour classes are absent after migration

- **WHEN** the post-merge audit runs `rg -n "neutral-[0-9]{3}" portal/src/`
- **THEN** zero matches are reported

#### Scenario: App-rendered surface uses the App's Design System

- **GIVEN** an App `app-1` with `design_system_ref=desing-system-2@1.0.0`
- **WHEN** the Portal renders `app-1`'s surface
- **THEN** the generated bundle MUST inline `desing-system-2`'s token sheet at the top of `globals.css`
- **AND** no other token sheet SHALL be loaded at runtime

## ADDED Requirements

### Requirement: Build-time resolution of `design_system_ref`

The Portal build pipeline SHALL resolve `App.design_system_ref` at build time, fetch the manifest from the Registry, verify the sha256 digest of the token sheet, fonts and component pack against the manifest, and inline the resolved artefacts into the App's bundle. The build SHALL fail loud if the manifest, the digest or the asset's `lifecycle_state` is not `approved`.

#### Scenario: Build fails when asset is not approved

- **GIVEN** an App with `design_system_ref=desing-system-3@2.1.0` and the asset version is in `lifecycle_state=in_review`
- **WHEN** the Portal build runs for this App
- **THEN** the build MUST fail with `design_system_asset_not_approved` and the message MUST include the asset id and current lifecycle state

#### Scenario: Build fails on digest mismatch

- **GIVEN** a Design System manifest whose `manifest.tokens_sha256` does not match the downloaded token sheet
- **WHEN** the Portal build runs
- **THEN** the build MUST fail with `design_system_digest_mismatch` and refuse to inline the sheet

### Requirement: Tenant-global visibility of built-in templates

The four built-in templates (`desing-system-1..4`) and the `ds-forge-default` alias SHALL be visible to every tenant as built-ins, regardless of the tenant's asset visibility configuration. Tenant-published Design Systems SHALL follow the normal Workspace/Tenant visibility rules of the Registry.

#### Scenario: Built-ins appear in every tenant's catalog

- **GIVEN** any tenant `t-1` and a member `user-1`
- **WHEN** `user-1` lists Design Systems via `GET /v1/design-systems`
- **THEN** the response MUST include the four built-in templates with `visibility=tenant_global`

#### Scenario: Tenant-published Design System is private by default

- **GIVEN** a Design System published by `tenant=t-1, workspace=ws-1` with default visibility
- **WHEN** `user-2` of `tenant=t-2` lists Design Systems
- **THEN** the published asset MUST NOT appear in `user-2`'s catalog
