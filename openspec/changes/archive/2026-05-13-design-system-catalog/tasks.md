## 1. Registry: new asset type

- [x] 1.1 Add `design_system` to the asset-type enumeration in the Registry schema and validation layer
- [x] 1.2 Add `manifest.tokens`, `manifest.components`, `manifest.fonts`, `manifest.screenshots`, `manifest.use_case` to the asset record; enforce sha256 pinning on tokens/components/screenshots URLs
- [x] 1.3 Update `contracts/openapi/registry.yaml` with the `design_system` type and the new manifest schema
- [x] 1.4 Add validation that rejects asset submissions missing any design-system manifest field
- [x] 1.5 Add eval thresholds: `accessibility>=0.9`, `brand_fidelity>=0.8` to gate approval; integrate with the existing eval harness
- [x] 1.6 Emit `asset.design_system.published.v1`, `asset.design_system.version_published.v1`, `asset.design_system.alias_changed.v1`

## 2. Sanity validator for tenant Design Systems

- [x] 2.1 Implement the validator: reject `url(...)` referencing off-tenant domains, ban `expression()`, JavaScript URI schemes, and layout-namespace collisions
- [x] 2.2 Wire the validator into the `proposed → approved` lifecycle transition for non-built-in Design Systems
- [x] 2.3 Add a `built_in_template=true` bypass for the four catalog templates with audit-recorded reason
- [x] 2.4 Unit tests: passing tokens, off-tenant URL, JavaScript scheme, layout collision, expression()

## 3. Built-in Design System templates

- [x] 3.1 Author `desing-system-1` package: wrap the current Portal token sheet, component pack and font preload list into the new manifest format
- [x] 3.2 Author `desing-system-2` package (corporate look-and-feel): tokens, component pack, fonts, screenshots, use_case
- [x] 3.3 Author `desing-system-3` package (minimal): tokens, component pack, fonts, screenshots, use_case
- [x] 3.4 Author `desing-system-4` package (marketing): tokens, component pack, fonts, screenshots, use_case
- [x] 3.5 Run Axe + brand fidelity rubric on each template; record `eval_scores` and ensure thresholds are met
- [x] 3.6 Publish all four to the platform tenant with `visibility=tenant_global`, `trust_level=T3`, `lifecycle_state=approved`
- [x] 3.7 Register the permanent alias `ds-forge-default → desing-system-1@<latest>`

## 4. Application entity: `design_system_ref` + overrides

- [x] 4.1 Add columns `design_system_ref` (text, NOT NULL with default `ds-forge-default`) and `design_system_overrides` (JSONB, default `{}`) to the `application` table; ship migration
- [x] 4.2 Update App create/patch payload schemas; validate `design_system_ref` resolves to an approved asset on every write
- [x] 4.3 Implement `POST /v1/apps/{id}/design-system:swap` (open PR, validate target approved, emit `app.design_system.swap_requested.v1`); auto-close prior open swap PR
- [x] 4.4 Implement `PATCH /v1/apps/{id}/design-system/overrides`; enforce surface-tokens-only namespace whitelist; emit `app.design_system.override_changed.v1`
- [x] 4.5 Implement webhook listener on the App's portal-bundle repo: on merged swap PR, flip `design_system_ref` and emit `app.design_system.changed.v1`
- [x] 4.6 Integration tests against ephemeral GitHub mock for the PR open/close/merge flow

## 5. Build-time merger

- [x] 5.1 Implement the Portal build hook that resolves `design_system_ref` and fetches the manifest from the Registry
- [x] 5.2 Verify sha256 digests of tokens, components, screenshots, fonts; fail loud on mismatch
- [x] 5.3 Merge per-component overrides: enforce surface-tokens-only namespaces; reject layout-token overrides at build time
- [x] 5.4 Regenerate `tailwind.config.js` token bindings from the merged token sheet
- [x] 5.5 Regenerate font preload manifest from `manifest.fonts`
- [x] 5.6 Snapshot tests: render the canonical sample composition under each of the four built-in templates and assert pixel diff = 0 against baselines

## 6. Wizard: Design System step

- [x] 6.1 Add the `design_system` step to the wizard state machine (visible only on the "create a new App" branch)
- [x] 6.2 Build the selection UI: four cards with screenshots, use-case copy, accessibility badge
- [x] 6.3 Build the live preview panel: side-by-side light/dark sample composition in a sandboxed DOM, applying the focused template's tokens
- [x] 6.4 Persist `design_system_ref` on the draft App; default to `ds-forge-default`
- [x] 6.5 Translate UI copy to ES/EN
- [x] 6.6 E2E test: complete wizard flow with each of the four templates; assert App is created with the correct `design_system_ref`

## 7. App Settings: swap UI

- [x] 7.1 Add a "Design System" panel in the App Settings tab (only visible to `app#owner`)
- [x] 7.2 Show the current `design_system_ref`, the list of approved templates available to the tenant, and the swap action
- [x] 7.3 Confirmation modal showing the diff (current vs target) and the policy gate (requires PR + CI)
- [x] 7.4 Surface the open swap PR with a deep link and status updates from the App's portal-bundle CI
- [x] 7.5 Show per-component overrides as an advanced section with the canonical primitive list

## 8. Migration

- [x] 8.1 Backfill `design_system_ref = ds-forge-default` for every existing App created before this change
- [x] 8.2 Verify visual parity: snapshot-diff the current Portal admin surfaces and the App-rendered surfaces against the new `desing-system-1`-driven build; expect zero diff for default
- [x] 8.3 Document the migration in the runbook

## 9. CLI: `forge design-system ...`

- [x] 9.1 `forge design-system list [--tenant]` — list catalog entries
- [x] 9.2 `forge design-system preview <ref>` — render the sample composition in a local browser
- [x] 9.3 `forge design-system install <ref> --app <id>` — apply via PR
- [x] 9.4 `forge design-system swap <app> --to <ref>` — convenience wrapper
- [x] 9.5 CLI integration tests

## 10. Observability and rollout

- [x] 10.1 Add `forge.design_system_catalog.enabled` feature flag (per-tenant), default `false`
- [x] 10.2 Dashboards: Design System adoption per template, swap PR open/merge rates, override usage
- [x] 10.3 SLO: swap PR is open within 30s of API call; build-time merger adds <500ms to portal builds
- [x] 10.4 Runbook: roll back a misbehaving Design System version by re-targeting the alias
- [x] 10.5 Per-tenant pilot: enable the flag for the platform tenant first, then for two pilot tenants, then globally
