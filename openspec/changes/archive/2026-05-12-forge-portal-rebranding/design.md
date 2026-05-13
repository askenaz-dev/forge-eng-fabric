## Context

The Portal today is a Next.js 14 (App Router) + Tailwind 3.4 project under `portal/` with NextAuth-based session management and 25+ pages that proxy server-side calls to the platform services (`control-plane`, `approvals`, `sdlc-orchestrator`, `runtime-registry`, `deploy-orchestrator`, `audit`, `asset-registry`, `openspec`, `incident-detection`, `evolution-loop`, `finops-recommendations`). Visual treatment is uniform Tailwind neutral palette, system font, flat sidebar — incompatible with the brand defined in `design/`.

The brand assets are already shipped as a complete reference implementation:

- `design/Forge Brand Notebook _standalone_.html`: 16-chapter brand notebook documenting tokens, type, logo, voice, components and the lattice/weave motif.
- `design/Forge Portal _standalone_.html`: a working React + raw-CSS implementation of the target Portal (sidebar, top bar, dashboard, run sheet) with ES/EN i18n, light/dark theme, density support and a global ⌘K-ready architecture.
- `design/fabric-unzipped/`: the unzipped source — `styles.css` (notebook), `portal-styles.css` (~1k lines, target Portal stylesheet), `portal-app.jsx` (target Portal React tree), `portal-data.jsx` (i18n dictionary + sample fixtures used for the reference), `portal-icons.jsx` (the Forge icon set), `tweaks-panel.jsx` (live theme/density tweaker for the design review), `app.jsx` / `i18n.jsx` / `icons.jsx` / `sections-foundations.jsx` / `sections-system.jsx` (notebook React tree).

We use the unzipped sources as our **visual ground truth and architectural blueprint** — copying tokens, class names and DOM structure when they match our service contracts, and rewriting the data layer to call real APIs instead of the mocks shown in `portal-data.jsx`.

Stakeholders: Platform Eng (owns the Portal codebase), Design (owns the brand notebook), DevEx (owns developer onboarding and ⌘K affordances), Security (owns OPA / approvals UX), Product Marketing (owns ES/EN copy parity).

Constraints:
- No CSS-in-JS runtime (build-time performance budget already tight).
- Must remain compatible with existing NextAuth + server-action data flow.
- Must keep server-rendered Tailwind class generation working for SEO and Lighthouse.
- All visible network traffic must remain inside the cluster — no external font / icon CDNs.
- Production cluster has internal `audit-stream` and `notifications` services for SSE.
- Tests run against `make dev-up` (Docker compose) — every flow must work end-to-end without MSW.

## Goals / Non-Goals

**Goals:**

- Land the Forge Engineering Fabric brand on every Portal page with zero behavioural regression.
- Ship a tested, accessible component library that downstream features build on.
- Replace flat Tailwind neutrals with the Ember palette, Instrument Serif display type, Geist UI type and JetBrains Mono.
- Add a real theme system (light / dark / system), density system (compact / comfortable / spacious) and ES/EN i18n with default ES.
- Introduce a global ⌘K command palette wired to the real platform services.
- Rebuild the dashboard to match the Brand Notebook screenshots — using real data, never mocks.
- Establish a Playwright visual + a11y baseline that runs in CI against a live `make dev-up` cluster.

**Non-Goals:**

- Adding a Storybook publication site (deferred).
- Replacing NextAuth with a different auth provider.
- Re-architecting server actions / data-fetching beyond what the new shell requires.
- Mobile screens below 720px width.
- Translating the marketing landing site (outside `portal/`).
- Switching to a CSS-in-JS runtime (styled-components, emotion).
- Introducing a state-management library — `useState` + React Query (already configured indirectly through Next caching) is enough.
- Rewriting the Workflow visual editor — only re-skin its chrome.

## Decisions

### 1. CSS variables as the canonical token surface, Tailwind as a thin utility bridge

We will declare every brand token (colour, type, spacing, radii, shadow, density unit `--u`) as CSS custom properties on `:root`, with `:root[data-theme="dark"]` and `:root[data-density="…"]` overrides. Tailwind will extend its theme to consume these variables (`colors: { primary: 'var(--primary)', 'fg': 'var(--fg)', 'bg-card': 'var(--bg-card)' … }`) and `darkMode` will switch to `['class', '[data-theme="dark"]']`. Custom component classes (`btn`, `btn--primary`, `card`, `kpi`, `chip`, `seg`, `sheet`, `terminal`) live in `globals.css`.

**Why over alternatives:**
- *CSS-in-JS (styled-components/emotion):* Adds runtime cost, breaks server rendering with App Router server components, and forks the token system per component. Rejected.
- *Pure Tailwind with arbitrary values:* Would force everyone to repeat `bg-[#DC4318]` everywhere, lose theme awareness, and explode the class soup. Rejected.
- *Stitches / Vanilla Extract:* Better than runtime CSS-in-JS but adds a custom toolchain that the team would have to learn just to do what CSS variables already do. Rejected.
- *Tailwind + design-token plugin:* Acceptable but more indirection than CSS variables + `theme.extend`. We choose the simpler path.

CSS variables are well-supported, server-renderable, theme-aware natively, and let us share tokens with non-Tailwind code (the SVG sparklines reference `var(--primary)` directly).

### 2. Self-hosted fonts loaded with `next/font/local`

We will use Next.js `next/font/local` to declare Instrument Serif, Geist and JetBrains Mono. Font binaries live in `portal/public/fonts/`. Fonts will be `preload`ed and their CSS variables hooked into the `--f-display`, `--f-sans`, `--f-mono` tokens.

**Why:**
- Eliminates external CDN dependencies (legal and offline-cluster requirement).
- `next/font/local` generates `font-display: swap` + size-adjust descriptors to mitigate FOIT/FOUT.
- Setting font CSS variables on `<html>` keeps the rest of the stylesheet token-agnostic.

Trade-off: bundle size grows by ~200KB (woff2). Acceptable.

### 3. Theme persistence: localStorage + server preference

We will store theme/density/lang preferences in two places:

1. `localStorage` (`forge_theme`, `forge_density`, `forge_lang`) for instant client-side application.
2. The control-plane user-preferences API (`POST /api/i18n/preference`, `POST /api/theme/preference`, `POST /api/density/preference`) for server-rendered initial paint and cross-device parity.

A server-side `cookie` named `forge_prefs` will mirror the same values so the initial Next.js render under App Router can set `<html data-theme data-density lang>` correctly without a flash.

**Why over alternatives:**
- *localStorage only:* Causes FOUC because Next renders before client JS runs.
- *Server-only:* Adds a round-trip on every preference toggle and breaks offline scenarios.
- *Cookie only:* No problem, but the localStorage layer lets us do client-only paths (Storybook, isolated tests) without a backend.

The dual-write is small: an `onChange` handler on the preference provider fires the POST and writes to localStorage in parallel; if the POST fails, the local cache still works and the SSE reconcile reconciles on next session.

### 4. Theme transition: data-theme-changing transient attribute

Switching from light to dark with default Tailwind transition rules causes a brief mid-flight colour blend (CSS interpolates the old and new `background-color` values). We will follow the brand notebook's pattern: set `data-theme-changing=""` on `<html>`, swap `data-theme`, then remove the attribute after two `requestAnimationFrame` ticks. While `data-theme-changing` is present, a CSS rule disables all transitions and animations.

**Why:**
- Zero JS animation library required.
- Reuses the design's already-validated approach (see `portal-app.jsx` `useTheme` hook).
- No interference with hover transitions after the swap completes.

### 5. ⌘K command palette built on cmdk + Radix Dialog

`cmdk` (Pacocoursey) is the best-in-class command palette primitive (used by Vercel, Linear, Raycast Web). We will wrap it in a Radix `Dialog` for the focus-trap and scrim, style it with the design system, and feed it via a typed source registry (`portal/src/components/palette/sources/*.ts`). Each source is async, parallel-fetched, capped per group, and ranked client-side by a simple `score = matchScore * weight + recency`.

**Why over alternatives:**
- *Building from scratch:* High risk for a11y bugs; `cmdk` solves the keyboard / aria-activedescendant correctness.
- *Algolia DocSearch widget:* Coupled to Algolia indexing; we don't want to push our data offsite.
- *Headless UI Combobox:* Less feature-complete than `cmdk` for the command-palette pattern.

The source registry pattern lets each platform team add their own sources later (e.g. `policies`, `incidents`) without touching the palette core.

### 6. Page-level migration: shell first, pages incrementally

We will land the new shell on a feature-flag (`PORTAL_REBRAND=1` env var → `useShellV2()` hook) for the first two weeks so the migration of the 25 existing pages can land progressively, page-by-page, behind the same flag. The default in `make dev-up` and staging will be `1`. Production cuts over when all pages are migrated. Once cutover completes, the flag and the legacy shell are removed in the follow-up archive change.

**Why:**
- Keeps trunk green during a multi-PR migration.
- Lets us run visual-regression baselines per page without breaking the rest.
- Avoids the giant-PR review problem (each page migrates in its own PR, ~200–400 LoC).

Trade-off: temporary code duplication of `layout.tsx` (V1 and V2). Acceptable because removal is mechanical and scoped.

### 7. Real data binding policy: enforced by lint + CI grep

We will enforce the "no mocks" rule via:

- A custom ESLint rule `forge/no-portal-mocks` that flags identifiers `mock_`, `fixture`, `fake_`, inline arrays of object literals matching a "looks-like-fixture" heuristic (>3 entries with `id:` and `title:` keys), and imports from `portal-data.jsx` or any file under `design/`.
- A CI grep step (`scripts/audit-no-mocks.sh`) that runs `rg` for forbidden tokens after build.
- A reviewer checklist item in `.github/PULL_REQUEST_TEMPLATE.md`.

**Why:**
- Lint catches at write time; CI grep catches everything else; reviewer is the last gate.
- The reference `portal-data.jsx` is *useful* as a shape contract but must never ship; flagging imports from `design/` makes that automatic.

### 8. Data fetching: prefer server components + route handlers

Existing pages use server-side `getServerSession` + `fetch` from server components. We will keep that pattern. Where the new components need client interactivity (KPI sparkline animations, theme toggles, command palette, run sheet), we will use `"use client"` and read the initial data via props from the server component. Client-side mutation goes through Next.js Route Handlers under `portal/src/app/api/*/route.ts` to add the `x-correlation-id` header and proxy NextAuth tokens.

**Why:**
- Matches existing project conventions (`/approvals/page.tsx`, `/runtimes/page.tsx`).
- Avoids leaking access tokens to the browser.
- Keeps the same observability instrumentation already in `instrumentation.ts`.

SSE for live updates uses a `/api/notifications/stream` route handler that proxies the `audit-stream` service over EventSource, filtering events by the principal's tenant/workspace.

### 9. Toast / SSE rail: one provider, multiple sources

A single `<ToastProvider>` exposes `useToast()` and also subscribes to `/api/notifications/stream` on mount. Toasts come from two sources:

- Imperative: any component can `toast.success(t("toast_theme"))`.
- Reactive: SSE events get translated into toasts via a typed mapping table.

**Why:**
- Avoids two competing toast libraries.
- Reactive mapping in one place means we audit it as a single concern.

The provider deduplicates toasts within a 500ms window (same key + same target) to prevent burst floods.

### 10. Testing strategy: Playwright e2e against real stack + axe a11y + visual baselines

We use the existing Playwright setup (`portal/playwright.config.ts`). New flows:

- **e2e**: dashboard.spec.ts, theme.spec.ts, lang.spec.ts, command-palette.spec.ts, approvals.spec.ts, run-sheet.spec.ts — each runs against the `make dev-up` docker-compose cluster with seeded real data (`make seed-portal`).
- **a11y**: `@axe-core/playwright` on every page in both themes and both languages.
- **visual**: `expect(page).toHaveScreenshot()` baselines per theme × language × density for the canonical pages. Baselines committed to `portal/tests/visual/__screenshots__/`.

We do *not* use MSW or any handler-mocking library — by design.

**Why:**
- Detects regressions in real integration paths.
- Matches the "no mocks" policy in tests too.
- Visual baselines catch design drift the type system can't see.

Cost: CI runs ~3 minutes longer. Acceptable.

### 11. Iconography: extract Forge PI set as React components

The `portal-icons.jsx` and `icons.jsx` files in `design/fabric-unzipped/` define a `PI.*` object of inline SVG icons. We will port these to per-icon `.tsx` files under `portal/src/components/icons/` so they tree-shake, follow our import conventions, and accept standard SVG props.

**Why:**
- Avoids `import * as PI` (no tree-shaking).
- TypeScript autocompletion on icon names.
- We can later inline-optimise (remove duplicate paths, replace with a single sprite) without changing call sites.

We will not pull in `lucide-react`, `heroicons` or `phosphor-react`: the brand has its own glyph set.

### 12. Forbid feature flags inside components

Once the rebrand cuts over, the shell migration flag is removed. Beyond that, we will not introduce feature flags inside components — components either exist in the new system or they don't. New "experiments" (e.g. the workflow editor variants) get their own route under `/labs/*` instead of branching inside a shared component.

**Why:**
- Code we already wrote in this project has a habit of accumulating dead branches behind flags.
- Visual-regression tests can't easily cover every flag combination.
- Simpler to read for new contributors.

### 13. Audit events for Portal UX actions

We will emit four new audit event types from Portal Route Handlers:

- `portal.theme.changed { from, to }`
- `portal.lang.changed { from, to }`
- `portal.density.changed { from, to }`
- `portal.command.invoked { source, target_id, query }`

These flow through the existing audit pipeline (Kafka topic `audit.events`) using the existing `audit` service. They do not require new database tables.

**Why:**
- Compliance ask: every user-facing toggle that affects what they see should be auditable for forensic reasons.
- Useful product analytics: knowing which commands are run informs the source-weighting algorithm in the palette.

## Risks / Trade-offs

- **Risk: Multi-PR migration leaves trunk in an inconsistent visual state for 1–2 sprints.**
  → Mitigation: feature flag (`PORTAL_REBRAND=1`) keeps users on V1 in production until cutover; staging always runs V2 so we catch regressions early.

- **Risk: Real-data tests are slower and flakier than mocked tests.**
  → Mitigation: dedicated `portal-e2e` job with retries=1 and per-test 30s timeout; flaky tests get a rerun-on-failure budget; seed scripts (`make seed-portal`) are idempotent and deterministic; tests share a session-scoped cluster instance.

- **Risk: Self-hosted fonts increase bundle by ~200KB and slow first paint.**
  → Mitigation: `next/font/local` with `display: swap`, preload of the two display-critical weights only (Instrument Serif Regular + Italic), Geist Regular and JetBrains Mono Regular; deferred load of medium weight and JetBrains Mono Italic.

- **Risk: Adding ⌘K shortcut conflicts with browser/extension shortcuts.**
  → Mitigation: only bind at document-level and only when no input/contenteditable is focused; document the conflict possibility in the docs; offer the search button as an alternative entry.

- **Risk: Theme transition guard (`data-theme-changing`) accidentally disables transitions for other state changes if not removed.**
  → Mitigation: use a `requestAnimationFrame` pair to remove it; add a Playwright test that asserts hover transitions still animate immediately after a theme swap.

- **Risk: i18n key drift between ES and EN over time.**
  → Mitigation: `i18n:check` script in CI; PR template item that authors confirm parity; type-system makes `t(unknownKey)` a compile error.

- **Risk: ESLint custom rule `forge/no-portal-mocks` produces false positives on legitimate static enums (status badges, severity levels).**
  → Mitigation: rule whitelists the allowed file paths (`src/components/primitives/`), narrows the heuristic to identifier patterns and uses an opt-out comment `// portal-mock-ok:reason`.

- **Risk: SSE connection fan-out under load (`/api/notifications/stream` proxying audit-stream) can saturate Node memory.**
  → Mitigation: per-process connection cap (256 concurrent), keepalive pings every 25s, server-side connection backpressure via Node's `pipeline`; fall back to 30s polling if SSE handshake fails.

- **Trade-off: The Brand Notebook is desktop-first; we explicitly defer mobile.**
  → Acceptable: the Portal is a developer/operator tool — primary usage is desktop. Mobile becomes its own future change.

- **Trade-off: Density × theme × lang × page × component matrix produces ~200 visual baselines.**
  → Acceptable: baselines are auto-generated on baseline-update PR; reviewers approve the screenshot diff explicitly.

## Migration Plan

**Phase 0 — Foundations (Week 1, behind `PORTAL_REBRAND=1`)**
- Add design-system stylesheet (`globals.css` token sheet + component layer).
- Configure Tailwind to consume tokens.
- Add `next/font/local` declarations + place font binaries in `portal/public/fonts/`.
- Add `ThemeProvider`, `LangProvider`, `DensityProvider`, `ToastProvider`, `CommandPaletteProvider`.
- Add new shell (sidebar + top bar + main).
- New `layout.tsx` mounts V2 shell when flag is set, otherwise V1.

**Phase 1 — Dashboard (Week 2)**
- Replace `app/page.tsx` with the Tablero (KPI grid + Runs + Approvals + Activity + Service mesh + Run sheet).
- Add `/api/observability/kpis`, `/api/notifications/stream`, `/api/command-palette/search`, `/api/i18n|theme|density/preference` route handlers.
- Land Playwright dashboard.spec + theme.spec + lang.spec + command-palette.spec.

**Phase 2 — Page migration (Weeks 3–4)**
- Migrate pages in this order (highest visibility first): `/approvals`, `/onboarding`, `/openspecs`, `/workflows`, `/marketplace`, `/incidents`, `/deployments`, `/drift`, `/evolution`, `/finops-recommendations`, `/kill-switch`, `/permissions`, `/runtimes`, `/runtimes/[id]`, `/settings/github`, `/apps/new`, `/alfred`, `/alfred/wizard`, `/assets`, `/initiatives`, `/templates`, `/pr-gates`, `/workflows/editor`, `/workspaces/new`.
- One PR per page; each PR adds a visual baseline and a passing axe scan.

**Phase 3 — Cutover (Week 5)**
- Flip default of `PORTAL_REBRAND` to `1` in production.
- Burn-in for 48h: monitor LCP, INP, CLS, error rate; rollback flag if regression > 10%.
- After burn-in passes, ship the follow-up archive change that deletes V1 layout, V1 styles, V1 pages and the feature flag.

**Phase 4 — Cleanup (Week 6, separate change proposal)**
- Remove the `PORTAL_REBRAND` flag and any V1 code paths.
- Publish `docs/portal/design-system.md`, `docs/portal/i18n.md`, `docs/portal/components.md`.

**Rollback strategy:**
- During Phases 0–2: `PORTAL_REBRAND=0` reverts to V1 instantly with a redeploy.
- During Phase 3 burn-in: flip the production env var; no code rollback needed for 48h.
- After Phase 4: standard git revert; design system stays available, just unused.

## Open Questions

- **Q1: Should the workflow visual editor (`/workflows/editor`) get the full re-skin or remain in a "lab" outside the rebrand scope?**
  Current lean: full re-skin (sidebar + topbar + token-aware node palette) because it's user-facing; the editor canvas itself is excluded.

- **Q2: Density preference — per-user or per-workspace?**
  Current lean: per-user (matches the brand notebook's UX); per-workspace adds complexity without clear value.

- **Q3: Should we support a `theme=auto` query parameter for screenshot tooling and customer support?**
  Current lean: yes, behind a feature flag; supports the Playwright visual baselines and the support team's "see what the user sees" workflow.

- **Q4: Should we replace the current `Forge Engineering Fabric` branding text in the auth pages too, or keep them untouched?**
  Current lean: replace the layout chrome (`/api/auth/signin` custom page) but keep NextAuth's default forms intact for now.

- **Q5: Lighthouse perf budget — do we enforce LCP < 1.5s or accept LCP < 2.5s during the migration?**
  Current lean: target 1.5s, enforce 2.5s in CI (warn-only at 1.5–2.5s); revisit after Phase 4.

- **Q6: Server-side preference storage — does the existing control-plane user-preferences endpoint already support `theme/lang/density`, or do we need to extend its schema?**
  Action: confirm with platform team; if missing, file a `user-preferences-extension` change to add `theme`, `lang`, `density`, `sidebar_collapsed` columns before Phase 0 begins.
