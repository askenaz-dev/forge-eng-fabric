## Why

The Forge Engineering Fabric portal currently ships a generic Tailwind shell (flat sidebar, monochrome neutral palette, default system fonts) that fails to communicate that the product is a pro-grade, AI-native engineering control plane. The `design/` folder and the `Forge - Engineering Fabric.zip` Brand Notebook describe a complete, mature design system — Ember/Instrument Serif typography, lattice-weave motifs, density/theme tokens, bilingual ES/EN copy, a sticky command bar with ⌘K, KPI cards with sparklines, segmented filter chips, and a structured nav split across Plataforma / Gobierno / Observabilidad / Cuenta — that is not implemented anywhere in `portal/src/`. We need to land the rebrand now so every downstream feature (Alfred wizard, approvals inbox, deployments console, evolution loop, marketplace) is built once on the canonical system instead of being retrofitted later. **Implementation MUST be 100% real**: all surfaces SHALL bind to the existing control-plane, approvals, openspec, sdlc-orchestrator, runtime-registry, deployment, incident, observability and audit APIs. Mock data, in-memory fixtures and `// TODO` placeholders are explicitly out of scope.

## What Changes

- **BREAKING**: Replace the current `portal/src/app/layout.tsx` shell (flat Tailwind sidebar with 25 module links and a single "sign out" header) with the Forge Portal shell: branded sidebar (logo + tenant pill + footer avatar), top bar (breadcrumb, ⌘K search, ES/EN pill, theme menu, notifications, GitHub link) and grouped navigation (Plataforma / Gobierno / Observabilidad / Cuenta).
- **BREAKING**: Migrate the Tailwind-only styling to a token-driven design system. Tokens (color, type, spacing, radii, shadow, density) SHALL live in CSS variables with `:root[data-theme="light|dark"]` and `:root[data-density="compact|comfortable|spacious"]` variants, and Tailwind utilities SHALL consume them via the `tailwind.config.js` `theme.extend`. Component classes (`.btn`, `.btn--primary`, `.card`, `.kpi`, `.badge--ok`, `.side-link`, `.chip`, `.seg`, `.scrim`, `.sheet`, `.terminal`, `.code`) SHALL be available in `globals.css` and used by every page.
- Introduce the **Ember** primary palette (`--ember-50…900`, primary `#DC4318` light / `#FF6A33` dark) alongside semantic tokens (`--thread` success / `--spark` warn / `--rust` danger / `--info` / `--steel` / `--copper`), the **Instrument Serif** display family for page titles, KPI numbers and italicised emphasis, **Geist** for UI, and **JetBrains Mono** for code, IDs and eyebrow labels. Self-host these fonts (`portal/public/fonts/`) — no Google Fonts runtime calls.
- Add a **theme system** (`light` / `dark` / `system`) backed by `prefers-color-scheme`, persisted in `localStorage` and surfaced as a top-bar menu. Theme switches MUST not produce a flash of unstyled or wrong-themed content and MUST not animate cross-tone background transitions.
- Add **bilingual (ES default / EN) i18n** with the dictionary from `design/fabric-unzipped/i18n.jsx` (nav, top bar, KPIs, runs, approvals, activity, services mesh, theme menu, toasts, sheet labels), a `useLang()` hook persisted in `localStorage`, and an `ES/EN` pill in the top bar. All hard-coded English strings in current portal pages SHALL be replaced with translation keys.
- Replace the homepage workspaces grid with the **canonical Dashboard** (Tablero): time-of-day greeting + serif display headline with italic emphasis, "Lanzar workflow" / "Invitar equipo" CTAs, four KPI cards (Runs en curso, Éxito 24 h, p95 latencia, Horas ahorradas/semana) with real numbers + sparklines, "Runs recientes" table with status pulse dots and filter chips, "Cola de aprobación" stack with high-severity badges, "Actividad de la plataforma" timeline, and "Mesh de servicios" hub-spoke SVG with live health states.
- Add a **global ⌘K command palette** (route to any module, search agents / runs / skills / specs, switch tenant/workspace, toggle theme/density/language) bound to `Cmd+K` / `Ctrl+K`.
- Rework the workspace / tenant switcher: branded tenant pill in the sidebar header (`PI.diamond` icon + tenant slug) wired to `/v1/workspaces`, with an inline picker that updates breadcrumbs and revalidates queries on change.
- Re-skin every existing page (`/approvals`, `/onboarding`, `/openspecs`, `/workflows`, `/workflows/editor`, `/marketplace`, `/incidents`, `/deployments`, `/drift`, `/evolution`, `/finops-recommendations`, `/kill-switch`, `/permissions`, `/runtimes`, `/runtimes/[id]`, `/settings/github`, `/apps/new`, `/alfred`, `/alfred/wizard`, `/assets`, `/initiatives`, `/templates`, `/pr-gates`, `/workspaces/new`) onto the new shell, KPI/card/badge primitives, and serif page heads. Page-specific functional logic and server actions SHALL be preserved 1:1.
- Add a **right-side detail Sheet** primitive (scrim + sheet) used by Runs, Approvals and Deployments for drill-down (workflow steps, signed evidence, proposed diff) — all data sourced from the existing services (`sdlc-orchestrator`, `approvals`, `deploy-orchestrator`, `audit`).
- Add a **density toggle** (compact/comfortable/spacious) in Settings, scoped to the authenticated user.
- Add a **toast / notification rail** for theme/language/approval/deploy/policy events, fed by Server-Sent Events from the existing audit stream.
- Add a **Playwright visual + a11y test suite** that exercises the dashboard, approvals queue, run sheet and theme/lang/density toggles against the live control-plane stack (no MSW, no fixtures): smoke run `make portal-e2e` SHALL succeed against a `make dev-up` cluster.
- **Non-goals** (explicitly out of scope): redesigning marketing pages outside `portal/`; introducing a CSS-in-JS runtime (styled-components / emotion); rewriting Next.js routes to the `pages/` router; replacing NextAuth; adding new backend capabilities; mocking any data source.

## Capabilities

### New Capabilities

- `portal-design-system`: Self-hosted Forge brand tokens (color, typography, spacing, radii, shadow, density), CSS-variable theme contract (light/dark/system + compact/comfortable/spacious), Tailwind config bridge, and the canonical component library (buttons, badges, cards, KPI, runs row, approval card, chips, segmented control, sheet, scrim, terminal, code block, toast) consumed by every Portal route.
- `portal-shell`: Persistent app shell — branded sidebar with grouped navigation (Plataforma / Gobierno / Observabilidad / Cuenta) + tenant pill + user footer; top bar with breadcrumb, ⌘K command palette trigger, ES/EN pill, theme menu, notifications and GitHub link — surfaced on every authenticated route and aware of route-level capability/permission guards.
- `portal-i18n`: Bilingual Spanish/English content layer for the Portal with default ES, persisted per-user preference, an `useLang()` hook, a `t(key, vars)` formatter consistent with the Brand Notebook dictionary, and a `lang` attribute applied on `<html>` for accessibility and screen-reader pronunciation.
- `portal-command-palette`: Global ⌘K (Mac) / Ctrl K (other) command palette that searches navigation routes, agents, runs, skills, specs and approvals; switches tenant/workspace; and toggles theme, density and language — all via existing portal data sources.
- `portal-dashboard`: Canonical Tablero landing page with time-of-day greeting, serif italic-accent headline, KPI grid (runs in flight, 24h success, p95 latency, hours saved/week) with sparklines, recent runs with status pulses and filter chips, approval queue with severity badges and timers, signed audit-log timeline, and live service-mesh visualization — all wired to live platform APIs.

### Modified Capabilities

<!-- No spec-level requirement changes for existing capabilities. Behavioural contracts of alfred-control-plane, openspec-backbone, approvals, deploy-orchestrator, etc. are preserved; only the Portal presentation layer changes. -->

## Impact

- **Code (Portal)**:
  - `portal/src/app/layout.tsx`: replaced by branded shell that mounts the sidebar, top bar, theme/lang providers and toast rail.
  - `portal/src/app/page.tsx`: replaced by the Tablero dashboard wired to control-plane, approvals, audit and sdlc-orchestrator endpoints.
  - `portal/src/app/globals.css`: replaced by the full design-system stylesheet (tokens + component classes).
  - `portal/src/app/providers.tsx`: extended with `<ThemeProvider>`, `<LangProvider>`, `<CommandPaletteProvider>` and `<ToastProvider>`.
  - New `portal/src/components/` directory: `shell/` (Sidebar, TopBar, ThemeMenu, LangPill, TenantPicker), `primitives/` (Button, Badge, Card, Kpi, Chip, Seg, Sheet, Scrim, Toast, Terminal, Code), `icons/` (Forge PI icon set extracted from `portal-icons.jsx`), `palette/` (CommandPalette), `runs/`, `approvals/`, `activity/`, `services/`.
  - New `portal/src/i18n/` directory: `dictionary.ts`, `useLang.ts`, `t.ts` translating the Brand Notebook dictionary.
  - Every existing page under `portal/src/app/*` updated to use the new primitives and shell (no Tailwind ad-hoc classes for surfaces / borders / shadows / typography).
  - `portal/src/app/api/` endpoints added or extended: `/api/i18n/preference`, `/api/theme/preference`, `/api/density/preference`, `/api/notifications/stream` (SSE proxy to audit-stream), `/api/command-palette/search` (proxies asset-registry, sdlc-orchestrator runs, approvals).
- **Assets**:
  - `portal/public/fonts/` adds self-hosted Instrument Serif (regular + italic), Geist (regular/medium/500), JetBrains Mono (regular + italic).
  - `portal/public/brand/forge-mark.svg` + favicon set (ICO, 16/32/192/512 PNG, Apple touch).
- **Configuration**:
  - `portal/tailwind.config.js`: extended with token mapping (colors, fontFamily, spacing, borderRadius, boxShadow), `darkMode: ['class', '[data-theme="dark"]']` and content globs for `src/components/`.
  - `portal/postcss.config.js`: unchanged but verified with Tailwind v3.4.
  - `portal/next.config.js`: enable `optimizeFonts: false` for self-hosted fonts and add `fonts` to the public asset cache headers.
- **Dependencies**:
  - Add `clsx` (or `classnames`) for class composition.
  - Add `cmdk` for the command-palette primitive (lightweight, accessibility-tested, used by Vercel/Linear).
  - Add `@radix-ui/react-dialog`, `@radix-ui/react-dropdown-menu`, `@radix-ui/react-tooltip`, `@radix-ui/react-tabs`, `@radix-ui/react-popover` for accessible primitives behind branded styles.
  - No CSS-in-JS runtime; no Google Fonts dependency.
- **Tests**:
  - `portal/tests/e2e/dashboard.spec.ts`, `portal/tests/e2e/theme.spec.ts`, `portal/tests/e2e/lang.spec.ts`, `portal/tests/e2e/command-palette.spec.ts`, `portal/tests/e2e/approvals.spec.ts`, `portal/tests/visual/*` (Playwright screenshot baselines per page × theme × lang × density) — all running against `make dev-up` services, no MSW, no fixtures.
- **Build & CI**:
  - `Makefile`: new target `make portal-rebrand-e2e` and Playwright integrated into existing `portal-test` lane.
  - GitHub Actions: matrix expanded to run e2e against the docker-compose dev stack (already used by other Portal flows).
- **Docs**:
  - New: `docs/portal/design-system.md` (token reference, do/don't, accessibility contracts), `docs/portal/i18n.md` (how to add a key, ES/EN parity rules), `docs/portal/components.md` (component inventory with props and live examples).
  - Updated: `docs/getting-started.md` (theme/lang toggle screenshots), `README.md` (Portal section pointing at the design system).
- **Audit / governance**: New audit event types emitted from Portal — `portal.theme.changed`, `portal.lang.changed`, `portal.density.changed`, `portal.command.invoked` — all carrying `actor`, `tenant`, `workspace`, `correlation_id`.
- **Out of scope (deferred to follow-up changes)**: Mobile breakpoints below 720px (Portal remains desktop-first); Storybook publication; brand expansion to marketing site; CSS Houdini paint-worklet for the weave background (the SVG `weave-bg` from the notebook is sufficient).
