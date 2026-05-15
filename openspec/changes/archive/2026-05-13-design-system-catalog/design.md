## Context

The Portal's design system today lives entirely in `portal/src/app/globals.css` plus `portal/tailwind.config.js`. Every Portal route consumes the same token surface, the same typography pipeline, the same component primitives. The brand notebook is "the brand" — there is no notion of multiple brands or themes beyond light/dark. This is fine for the platform admin surfaces, but it blocks two recurring asks:

1. **White-label Apps**: tenants want a B2B-internal App that does not look like the Forge admin console; they want a more conservative corporate palette, a different typography, less ember orange.
2. **Surface-specific themes**: a marketing surface wants a different feel from an internal dashboard, even within the same tenant.

The newly minted `app-first-class-entity` change introduces App as the anchor. We now need to (a) make Design System an installable, versioned asset, (b) let an App declare which one it uses, and (c) make the swap a controlled, audited, PR-driven event rather than a runtime style mutation.

Constraints:
- The Registry already supports five asset types. Adding a sixth is additive but touches schema, validation, lifecycle policy and the CLI.
- The Portal's CSS is already token-driven (every value is a CSS custom property bound to Tailwind utilities). We do not have to rewrite component primitives — we have to swap the token sheet and the font set.
- Per-component overrides MUST NOT break layout invariants (spacing scale, grid). They affect *surface tokens* (colour, typography, border radius) only.

Stakeholders: Portal team (owners of the design system stylesheet), Registry team (owners of asset typing), DX team (owners of `forge` CLI), App owners across pilot tenants (consumers).

## Goals / Non-Goals

**Goals:**
- Make Design System an installable asset type with the same lifecycle and trust contract as the existing five types.
- Ship four initial templates (`desing-system-1..4`) and document them.
- Let an App pick a Design System at creation, see a live preview before commit, and swap it later via a PR-gated change.
- Allow per-component overrides on an App without changing the App's base Design System.
- Keep the migration zero-visual-change for existing Apps (they get pinned to the current Portal design system).

**Non-Goals:**
- Building a visual design-token editor inside the Portal. Authoring a new Design System is an out-of-Portal flow (a packaged asset bundled by the design team and published via the Registry CLI). The Portal only consumes.
- Per-route Design System within the same App. Overrides are per-component, not per-route.
- Cross-tenant sharing of private Design Systems. Initial release: built-in templates are tenant-visible to all; tenant-published Design Systems are private to the publishing tenant.
- Custom font pipelines beyond the three families used today (Instrument Serif, Geist, JetBrains Mono); new templates pick from a curated font set.

## Decisions

### Decision 1 — `design_system` is a Registry asset type, not a portal-only manifest

**Choice**: add `design_system` to the Registry's enumerated asset types. The asset record carries the canonical metadata (description, screenshots, owner, lifecycle, trust level) plus a `manifest` block listing the token sheet URL, the component pack URL and the font preload list. Distribution uses the existing gateway-publish hook for skill/agent assets.

**Why**: it gives us free lifecycle management (proposed/in_review/approved/deprecated/retired), free eval-score hooks (we evaluate accessibility, contrast, brand fidelity), free supply-chain attestations (the token sheet bundle is signed) and free Workspace-vs-Tenant visibility rules. Inventing a parallel surface for design systems would re-implement all of that.

**Alternatives considered**:
- *Portal-only manifest stored under `portal/themes/`*. Rejected — no audit, no signing, no per-tenant visibility, no lifecycle.

### Decision 2 — Reference, not embed: `App.design_system_ref = asset_id@version`

**Choice**: the App stores a *versioned reference* to the Design System, never the manifest body. Resolution happens at build time when the portal bundle is generated for that App.

**Why**: keeps the App record small, lets us upgrade the Design System asset without rewriting every App row, makes the swap a single-field update + PR.

### Decision 3 — Runtime swap is a PR, not a hot-reload

**Choice**: when an owner picks a new Design System from the App Settings tab, the platform opens a PR against the App's repo updating `app.config.json` (the file that declares `design_system_ref`), regenerating the Tailwind config and the font preload manifest. Merging the PR triggers a normal CI/CD run. The Portal does not hot-swap stylesheets at runtime.

**Why**: it preserves traceability (the swap is in `git log`), aligns with existing deployment policies (`deploy:prod` requires approval if configured), and avoids the operational pain of mid-flight style mutation that would surprise users mid-session. The "PR opens automatically" UX keeps the friction low while preserving the policy gate.

**Alternatives considered**:
- *Live style swap via CSS variable injection*. Rejected — no audit, breaks current-user sessions, no rollback path.

### Decision 4 — Per-component overrides apply to surface tokens only

**Choice**: an App's `design_system_overrides` map MAY only override the *surface tokens* (colour, typography, border radius, shadow) of a primitive. Layout tokens (spacing scale, grid columns, breakpoints) MUST come from the App's base Design System. The runtime renderer enforces this by selectively merging only the allowed token namespaces.

**Why**: per-component theming is the realistic ask ("make my buttons feel corporate but keep the rest of the chrome"); per-component layout is a rabbit hole that would force every component to be tested across N grids.

### Decision 5 — The four built-in templates are platform-owned, T3, approved

**Choice**: `desing-system-1..4` are published as `design_system` assets in the platform tenant, owned by the `forge-platform-design` team, pinned at `trust_level=T3`, `lifecycle_state=approved` and shared with `visibility=tenant_global` so every tenant sees them as built-ins. Their identifiers are exact strings as supplied by the product owner; semantics are documented on each asset's README.

**Why**: respects the user's explicit naming decision (`Desing System 1, 2, 3, 4` — note the spelling); avoids renames that would invalidate references in early-pilot tenants. The README explains the look (default-forge / corporate / minimal / marketing) so the cryptic identifier is not a UX problem.

### Decision 6 — Existing Apps pin to the current Portal design system

**Choice**: a migration step assigns `design_system_ref = desing-system-1@<latest>` to every existing App (where `desing-system-1` is the package wrapping the current Portal design system tokens, fonts and component pack). No visual change.

**Why**: lets us flip the feature flag without surprising existing tenants. They can opt into a different Design System later via the swap PR.

### Decision 7 — `ds-forge-default` alias points at `desing-system-1`

**Choice**: the platform registers a permanent alias `ds-forge-default` that always points to the latest approved version of `desing-system-1`. Downstream tooling (the migration job, the App scaffolder) references the alias; this lets us re-anchor the alias to a different template later without rewriting App records.

**Why**: indirection is cheap and saves a migration if the product team decides to repackage the default.

## Risks / Trade-offs

- **[Risk] Per-component overrides produce visually broken UIs** when token namespaces are mismatched. → Mitigation: token namespaces are typed (`color-*`, `radius-*`, `font-*`, `shadow-*` are allowed; `space-*`, `grid-*`, `breakpoint-*` are NOT); the merge layer rejects un-allowed namespaces at build time.
- **[Risk] Swap PRs flood the App's repo**. → Mitigation: only an owner can trigger a swap; rate-limit one swap PR open at a time per App; auto-close the previous swap PR when a new one is opened.
- **[Risk] Tenant publishes a malicious token sheet** (CSS injection via custom properties). → Mitigation: every published Design System asset MUST pass a sanity validator that bans `url(...)` references to off-tenant domains, JavaScript-y values and CSS escapes; built-in templates skip the validator.
- **[Trade-off] The four template identifiers are misspelled (`Desing` not `Design`)**. → Acceptable: identifiers come from the product owner verbatim. Documentation describes the look-and-feel; the typo can be corrected with a future asset version (SemVer major).
- **[Trade-off] No in-Portal design token editor**. → Acceptable for Phase 5; the design team authors templates out-of-Portal and ships them as Registry assets.
- **[Trade-off] Swap requires PR + CI + deploy**. → Acceptable: alignment with deployment policies and audit; the lag (typically <10 minutes for a Portal redeploy) is justified by traceability.

## Migration Plan

1. **M0 — Code & schema**: ship `design_system` asset type, the four built-in template bundles, the App schema additions (`design_system_ref`, `design_system_overrides`).
2. **M1 — Catalogue publication**: publish `desing-system-1..4` to the platform tenant; set `tenant_global` visibility; register the `ds-forge-default` alias to `desing-system-1`.
3. **M2 — Backfill**: assign `design_system_ref = ds-forge-default@<latest>` to every existing App. No visual change.
4. **M3 — Wizard step**: enable the Design System selection step in the Intent Capture Wizard for new-App branches (gated by `forge.design_system_catalog.enabled`).
5. **M4 — Swap UX**: enable the swap action in App Settings.
6. **M5 — Per-component overrides**: enable the overrides API and the corresponding UI affordance.

**Rollback**: any step ≤ M2 rolls back by clearing `design_system_ref` and re-pointing to the hard-coded portal stylesheet. After M2, rollback is to disable the wizard step and the swap UX while keeping the asset in place.

## Open Questions

- Should we expose `forge design-system bench` (a CLI that runs Axe + Lighthouse against a Design System asset to score accessibility and performance)? Recommendation: yes, in a follow-up change.
- How do we present the four templates in the wizard preview? Strawman: side-by-side thumbnails of a sample run-card + KPI tile + button stack in both light/dark themes.
- Versioning policy for built-in templates: SemVer matches asset versioning; a minor bump must not change layout tokens. Confirmed during M1.
