## Why

Today the Forge Portal ships a **single, hard-coded** brand design system (`portal-design-system`): one token sheet, one typography pipeline, one component variant set. Every App rendered through the Portal — internal HR tools, customer-facing marketing surfaces, corporate intranet — inherits the same Forge brand chrome. That is fine for the platform's own admin surfaces, but it blocks tenants from white-labelling Apps, prevents a corporate look-and-feel for B2B internal tools, and makes "design system" a non-discoverable, non-swappable concept. Now that App is a first-class entity (`app-first-class-entity`), we have the anchor we need to attach a selectable Design System per App.

## What Changes

- Introduce **`design_system`** as a new asset type in the AI Asset Registry, sitting alongside the existing five types (MCP Server, Agent Skill, Agent, Workflow, Prompt Template). Six asset types after this change.
- Ship **four initial Design System templates** registered as `design_system` assets, owned by the platform team and pinned at trust level T3+: `desing-system-1`, `desing-system-2`, `desing-system-3`, `desing-system-4` (user-supplied identifiers; documentation explains the look-and-feel of each).
- Add `design_system_ref` to the **App** entity (introduced in `app-first-class-entity`): a versioned pointer to a Design System asset (`asset_id@version`). The reference SHALL be resolved at build time and inlined into the App's portal bundle.
- Extend the **Intent Capture Wizard** with a *Design System* selection step (only when the user is creating a new App) and a preview panel that renders sample components (button, card, KPI tile, run row) with the chosen tokens before commit.
- Expose a **runtime swap mechanism**: an App owner can change `design_system_ref` from the App Settings tab; the platform opens a PR against the App's portal bundle with the updated token sheet, regenerated Tailwind config and changed font preload list. The PR is policy-gated and audited; merging it triggers a fresh deployment.
- Allow **per-component overrides** at the App level: an App SHALL be able to declare `design_system_overrides: { button: "ds-3", card: "ds-1" }` to opt individual components into a different Design System without changing the App's base. Overrides SHALL render with their source Design System's tokens, but layout primitives (grid, spacing scale) SHALL remain the App's base.
- New Registry endpoints, events (`asset.design_system.published.v1`, `app.design_system.changed.v1`) and CLI commands for previewing, installing and swapping a Design System.

## Capabilities

### New Capabilities

- `design-system-catalog`: registry, versioning, distribution and lifecycle for Design System assets — the catalog of selectable tokens/themes/component packs, the four built-in templates, and the swap & override APIs.

### Modified Capabilities

- `ai-asset-registry`: add `design_system` to the supported asset types; add the metadata block (tokens, component pack, font set, screenshots, swap manifest) required for a Design System asset.
- `application-entity`: App SHALL carry `design_system_ref` (versioned) and an optional `design_system_overrides` map; expose endpoints to read/swap them.
- `intent-capture-wizard`: extend the wizard with a "Design System" step (visible only when creating a new App) including a live preview panel.
- `portal-design-system`: the Portal stylesheet is now one of N installable Design Systems, not the singleton — it remains the default but yields to the App's `design_system_ref` when present.

## Impact

- **Registry**: new `design_system` asset type, schema, validation rules and storage layout (token sheet + component pack archive). New publication event.
- **Application entity**: schema gains `design_system_ref` and `design_system_overrides`; events `app.created.v1`/`app.updated.v1` carry these fields.
- **Portal build**: the portal bundle becomes parameterisable by an installed Design System manifest at build time; runtime swap regenerates the bundle via a PR.
- **Intent Capture Wizard**: new step + preview component; copy in ES/EN.
- **CLI / forge-developer-cli**: add `forge design-system list|preview|install|swap` commands.
- **Migration**: every existing App created before this change SHALL be assigned `design_system_ref = ds-forge-default@<latest>` (i.e., the current Portal design system, registered as `desing-system-1`). No visual change for those Apps.
- **Documentation**: each of the four templates ships with a one-pager describing tone, suggested use case and screenshots. The four identifiers (`desing-system-1..4`) are placeholders that the platform team can rename later; their semantic meaning is documented per template.
