# portal-design-system Specification

## Purpose

The Forge Portal design system — token surface, typography pipeline, theme &
density systems, primitive component library, accessibility contract, Tailwind
bridge and the canonical CSS stylesheet — that every Portal route consumes.
Created by archiving change `forge-portal-rebranding`.

## Requirements

### Requirement: Token-driven brand palette

The Portal SHALL expose every brand colour as a CSS custom property declared on `:root` and overridden on `:root[data-theme="dark"]`. The palette SHALL include the full Ember ramp (`--ember-50` through `--ember-900`), the semantic tones `--thread` (success / verdigris), `--spark` (warning / amber), `--rust` (danger), `--info`, `--steel`, `--copper`, and the neutrals `--ink`, `--paper`, `--bg`, `--bg-2`, `--bg-card`, `--bg-elev`, `--bg-hover`, `--bg-active`, `--bg-sunk`, `--fg`, `--fg-2`, `--fg-3`, `--border`, `--border-2`, `--border-strong`. The token `--primary` SHALL resolve to `--ember-500` in light mode and `#FF6A33` in dark mode. No portal component MAY hard-code hex colours; every colour reference SHALL be either a CSS variable or a Tailwind utility bound to a variable via `tailwind.config.js`.

#### Scenario: Light theme uses Ember 500 as primary

- **WHEN** `:root` is in its default state (no `data-theme` attribute or `data-theme="light"`)
- **THEN** `getComputedStyle(document.documentElement).getPropertyValue('--primary')` returns `#DC4318`
- **AND** any element with `background: var(--primary)` renders with that exact colour

#### Scenario: Dark theme primary shifts to glowing ember

- **WHEN** `document.documentElement.setAttribute('data-theme', 'dark')` is applied
- **THEN** `--primary` resolves to `#FF6A33`
- **AND** `--bg` resolves to `#0F0D0B` and `--fg` to `#F2EDE2`
- **AND** the `--shadow-emb` token gains the ember-tinted glow from the design notebook

#### Scenario: No component contains a literal hex color

- **WHEN** the design-system lint rule (`stylelint-declaration-strict-value` with `color`/`background`/`border-color` scopes) runs over `portal/src/**/*.{css,tsx}`
- **THEN** the lint reports zero violations
- **AND** any newly authored hex literal under `portal/src/` outside `portal/src/app/globals.css` fails CI

### Requirement: Self-hosted typography pipeline

The Portal SHALL self-host the brand typefaces: **Instrument Serif** (regular + italic) for display, **Geist** (regular + medium + semibold) for UI, and **JetBrains Mono** (regular + italic) for monospace. The CSS variables `--f-display`, `--f-sans`, `--f-mono` SHALL map to these families with system fallbacks. Fonts SHALL be fetched at *build time* (via `next/font/google` for Instrument Serif and JetBrains Mono, and via the `geist` npm package for Geist) and inlined into the Next.js build output. No `fonts.googleapis.com` or `fonts.gstatic.com` request SHALL be issued by the Portal at runtime.

#### Scenario: Display text uses Instrument Serif

- **WHEN** the dashboard headline `<h1 class="page-title">` is rendered
- **THEN** its computed `font-family` resolves to `"Instrument Serif", "Cormorant Garamond", Georgia, serif`
- **AND** the network panel records the font being loaded from `/_next/static/media/`, not from any Google domain

#### Scenario: Monospace italic is available for OPA policy refs

- **WHEN** a run row renders `<span class="pol">deploy.prod.v2</span>` styled by `font-family: var(--f-mono); font-style: italic`
- **THEN** the JetBrains Mono italic face is loaded and the glyphs render correctly (no synthetic italic from the browser)

### Requirement: Theme system with prefers-color-scheme and persistence

The Portal SHALL support three theme preferences: `light`, `dark`, `system`. The effective theme SHALL be derived from `prefers-color-scheme` when the preference is `system`, otherwise from the explicit choice. The preference SHALL be persisted in `localStorage` under key `forge_theme`, and the resolved theme SHALL be applied as `data-theme` on `<html>`. The applied theme SHALL also be persisted server-side to the user's profile via `POST /api/theme/preference` so that the initial paint on subsequent sessions uses the correct theme without FOUC. Theme transitions SHALL suppress CSS transitions for the swap frame to avoid mid-animation colour artefacts (via a transient `data-theme-changing` attribute that disables transitions for two `requestAnimationFrame` ticks).

#### Scenario: System preference follows the operating system

- **WHEN** the user's stored preference is `system` and the OS reports `prefers-color-scheme: dark`
- **THEN** `<html>` is rendered with `data-theme="dark"` on initial paint
- **AND** changing the OS preference to light updates `data-theme` to `light` without a page reload

#### Scenario: Explicit dark choice persists across sessions

- **WHEN** the user selects "Oscuro" in the theme menu
- **THEN** `localStorage.forge_theme === 'dark'`, `<html data-theme="dark">`, and a `POST /api/theme/preference {"theme":"dark"}` is sent
- **AND** on the next session the initial server-rendered HTML already carries `data-theme="dark"` (no FOUC)

#### Scenario: Theme swap suppresses transition flicker

- **WHEN** the user toggles between light and dark
- **THEN** during the swap frame `<html>` carries `data-theme-changing=""`, all `* , *::before, *::after { transition: none; animation-duration: 0s }` rules apply
- **AND** after two `requestAnimationFrame` ticks the attribute is removed and hover transitions resume

### Requirement: Density system

The Portal SHALL expose three density preferences: `compact`, `comfortable` (default), `spacious`. The preference SHALL be applied as `data-density` on `:root` and SHALL modulate the base spacing unit `--u` (3 / 4 / 5 px) so that every component that uses `var(--s-1) … var(--s-24)` scales coherently. The preference SHALL be persisted in `localStorage` and via `POST /api/density/preference`.

#### Scenario: Compact density tightens the layout

- **WHEN** the user picks "Compact" in Settings
- **THEN** `<html data-density="compact">` is set, `--u` resolves to `3px`, and `--s-4` to `12px`
- **AND** sidebar links and KPI cards visibly tighten without overlap

#### Scenario: Density choice survives page navigation

- **WHEN** density is set to `spacious` and the user navigates from `/` to `/approvals`
- **THEN** the new page renders with `data-density="spacious"` from the first server-side paint

### Requirement: Component primitive library

The Portal SHALL ship a canonical component library covering buttons, badges, cards, KPI tiles, chips, segmented controls, side-sheet (scrim + sheet), toast, terminal, code block, run row, approval card and sidebar/topbar elements. Each primitive SHALL be implemented as a React component under `portal/src/components/primitives/` and styled by composable design-system CSS classes (`btn`, `btn--primary`, `badge`, `badge--ok`, `card`, `card-hd`, `kpi`, `chip`, `seg`, `scrim`, `sheet`, `terminal`, `code`). Variants and tones SHALL be type-safe (`type ButtonVariant = "primary" | "secondary" | "ghost" | "danger"`).

#### Scenario: Primary button matches the Brand Notebook

- **WHEN** `<Button variant="primary">Lanzar workflow</Button>` is rendered
- **THEN** the resulting DOM is `<button class="btn btn--primary">…</button>`
- **AND** computed styles match: background `var(--primary)`, color `var(--on-primary)`, border-radius `var(--r-2)`, height >= 32px
- **AND** the matching Playwright screenshot baseline at `portal/tests/visual/button-primary-light.png` is byte-identical (zero pixel diff)

#### Scenario: Status badge maps semantic tones to brand colours

- **WHEN** `<Badge tone="ok">Corriendo</Badge>` is rendered
- **THEN** the DOM is `<span class="badge badge--ok"><span class="dot" />Corriendo</span>`
- **AND** the `.dot` is filled with `var(--thread)` and the badge passes WCAG AA contrast against `--bg-card` in both themes

#### Scenario: Card body uses the canonical surface tokens

- **WHEN** any page renders `<Card><CardHeader title="Runs recientes" sub="últimos 50" /><CardBody>…</CardBody></Card>`
- **THEN** the wrapper carries `background: var(--bg-card)`, `border: 1px solid var(--border)`, `border-radius: var(--r-4)`
- **AND** the header title uses Instrument Serif italic, font-size 22px, letter-spacing -0.015em

### Requirement: Accessibility contract

Every primitive SHALL meet WCAG 2.1 AA. Specifically: interactive controls SHALL expose a visible focus ring (`box-shadow: var(--ring)`); colour SHALL never be the sole signal for state (status pulses pair with text or icons); the theme toggle SHALL not flash arbitrary colours on first paint; the sidebar nav SHALL be keyboard-navigable with `Tab` and `Shift+Tab` and exposed as a `<nav aria-label>`; modal sheets and the command palette SHALL trap focus and restore it on close; all icons used as buttons SHALL have an `aria-label`.

#### Scenario: All buttons have a visible focus ring

- **WHEN** any `.btn` receives keyboard focus
- **THEN** `:focus-visible` applies the `--ring` box-shadow at 3px width with `--primary`-tinted color-mix at 78% transparency

#### Scenario: Sheet traps focus

- **WHEN** the run detail sheet opens
- **THEN** focus moves to the sheet container, `Tab` cycles within the sheet, `Escape` closes it and returns focus to the originating row

#### Scenario: Axe scan reports no violations

- **WHEN** `@axe-core/playwright` runs against the dashboard, approvals page and run sheet in both themes and both languages
- **THEN** the violation count is zero for serious/critical rules

### Requirement: Tailwind config bridges to design tokens

`portal/tailwind.config.js` SHALL extend the theme so that `bg-ember-500`, `text-fg`, `border-border`, `shadow-pop`, `rounded-r-3`, `font-display`, `font-sans`, `font-mono`, etc. all resolve to the corresponding CSS variables. `darkMode` SHALL be configured as `['class', '[data-theme="dark"]']`. Content globs SHALL include `./src/components/**/*.{ts,tsx}` in addition to `./src/app/**/*.{ts,tsx}`. Tailwind utilities SHALL be used for layout (grid/flex/spacing) and design-system classes SHALL be used for surfaces, typography and chrome.

#### Scenario: Tailwind utility binds to design token

- **WHEN** a component uses the utility `bg-bg-card`
- **THEN** the generated CSS rule is `background-color: var(--bg-card)`
- **AND** swapping `data-theme="dark"` changes the rendered colour automatically without a re-build

### Requirement: Design-system stylesheet is the sole source of branded styles

`portal/src/app/globals.css` SHALL contain the entire token sheet and component layer. No other CSS file under `portal/src/` SHALL declare colour, font or shadow values; component-specific layout-only CSS modules are permitted if they consume only design-system tokens. The legacy classes used in the existing Tailwind shell (`bg-neutral-50`, `bg-white`, `dark:bg-neutral-900`, ad-hoc `rounded`, `border-neutral-*`) SHALL be removed.

#### Scenario: Legacy neutral colour classes are absent after migration

- **WHEN** the post-merge audit runs `rg -n "neutral-[0-9]{3}" portal/src/`
- **THEN** zero matches are reported

### Requirement: Real data binding only

Every component in the design system that displays data SHALL accept its values via typed props sourced from the existing platform services (`control-plane`, `approvals`, `audit`, `sdlc-orchestrator`, `runtime-registry`, `deploy-orchestrator`, `incident-detection`, `asset-registry`, `openspec`, `evolution-loop`, `finops-recommendations`, `policy-svc`, `openfga`). Inline placeholder objects, hard-coded mock arrays and `// TODO replace with real data` markers SHALL be rejected in code review.

#### Scenario: KPI card consumes a real metrics endpoint

- **WHEN** the Tablero renders the "p95 latencia" KPI
- **THEN** its `value` prop is the JSON field `p95_ms` from `GET /v1/observability/kpis?window=24h` proxied through `portal/src/app/api/observability/kpis/route.ts`
- **AND** the sparkline `data` prop is the `samples` array from the same response (12 buckets)

#### Scenario: Reviewer flags any inline mock array

- **WHEN** a PR introduces `const RUNS = [{ id: "wf_…" }, …]` inside `portal/src/`
- **THEN** `scripts/audit-no-mocks.sh` fails in CI and the reviewer rejects the change
