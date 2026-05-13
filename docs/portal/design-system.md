# Forge Portal — Design System

The Portal's visual language is defined by the **Forge Engineering Fabric** brand
notebook (`design/Forge Brand Notebook _standalone_.html`) and implemented in
`portal/src/app/globals.css`. This document is the developer reference.

## Token surface

All tokens live as CSS custom properties on `:root`. Tokens are overridden on
`:root[data-theme="dark"]` for dark mode and on `:root[data-density="…"]` for
density. **No portal source file (other than `globals.css` itself) should
declare a literal hex / rgb / hsl colour.** This is enforced by
`stylelint-declaration-strict-value` in CI.

### Colour

| Token            | Light value | Dark value | Purpose |
|------------------|------------|------------|---------|
| `--primary`      | `#DC4318`  | `#FF6A33`  | Ember — primary brand action |
| `--primary-h`    | `#B7330F`  | `#FF8253`  | Primary hover |
| `--on-primary`   | `#FFFFFF`  | `#0C0B0A`  | Foreground on primary |
| `--thread`       | `#4F8C76`  | `#6FBC9C`  | Success / verdigris |
| `--spark`        | `#E8B100`  | `#FFD24B`  | Warning / amber |
| `--rust`         | `#B33620`  | `#E45A40`  | Danger |
| `--info`         | `#2A6FDB`  | `#5A92E8`  | Info |
| `--steel`        | `#243441`  | `#6E8195`  | Steel accent |
| `--copper`       | `#B57340`  | `#D89466`  | Copper accent |
| `--ink`          | `#13110F`  | `#13110F`  | Ink (terminals / scrim) |
| `--paper`        | `#F5F1E9`  | `#F5F1E9`  | Paper (inverse surface) |
| `--fg`           | `#13110F`  | `#F2EDE2`  | Primary foreground |
| `--fg-2`         | `#57544F`  | `#B0A797`  | Secondary fg |
| `--fg-3`         | `#908A80`  | `#6F6758`  | Muted fg |
| `--bg`           | `#FAFAF7`  | `#0C0B0A`  | App background |
| `--bg-card`      | `#FFFFFF`  | `#141210`  | Card / elevated surface |
| `--bg-hover`     | `#F2EFE8`  | `#1A1714`  | Hover surface |
| `--bg-active`    | `#EBE7DD`  | `#221E1A`  | Active / pressed surface |
| `--bg-sunk`      | `#F2EFE8`  | `#0A0908`  | Sunken / inset surface |
| `--border`       | `#E5E0D2`  | `#221E1A`  | Default border |
| `--border-2`     | `#D9D2C5`  | `#2D2823`  | Stronger border |
| `--border-strong`| `#BBB2A0`  | `#423A32`  | Emphasised border |

The full Ember ramp `--ember-50 … --ember-900` is available for swatches and
gradients.

### Typography

| Token         | Family |
|---------------|--------|
| `--f-display` | Instrument Serif → Cormorant Garamond → Georgia → serif |
| `--f-sans`    | Geist → -apple-system → BlinkMacSystemFont → Segoe UI → system-ui |
| `--f-mono`    | JetBrains Mono → SF Mono → ui-monospace → Menlo |

Fonts are self-hosted from `portal/public/fonts/`. See the README in that
folder for the manifest. **No Google Fonts CDN requests are made at runtime.**

Usage:
- Headings, KPI numbers and italicised emphasis use the **display** family.
- Body, sidebar links, badges, buttons use the **sans** family.
- IDs, code, eyebrows, run policy slugs, mono-numerics use the **mono** family.

### Spacing

A base unit `--u` is set per density:

| Density       | `--u` |
|---------------|-------|
| `compact`     | `3px` |
| `comfortable` | `4px` (default) |
| `spacious`    | `5px` |

All other spacing tokens (`--s-1` through `--s-24`) are derived as multiples
of `--u`, so density swaps tighten or loosen the entire layout coherently.

### Radii

`--r-1` (4px), `--r-2` (6px), `--r-3` (8px), `--r-4` (12px), `--r-5` (16px),
`--r-pill` (999px). Buttons → `r-2`, cards → `r-4`, sheets → `r-5`.

### Shadows

`--shadow-1`, `--shadow-2`, `--shadow-pop`, `--shadow-emb`, `--ring`.

## Theme system

Three preferences, two effective resolutions:

- `light` — explicit.
- `dark`  — explicit.
- `system` — follows `prefers-color-scheme`.

Wiring:

1. The server reads the `forge_prefs` cookie via `src/lib/prefs.ts`.
2. `layout.tsx` applies `data-theme` to `<html>` when the preference is
   explicit; `system` is resolved on the client by `ThemeProvider`.
3. `ThemeProvider` writes a transient `data-theme-changing=""` attribute
   for one frame around every swap to suppress mid-flight colour
   interpolation.

## Density system

Three preferences applied via `data-density` on `<html>`. The `DensityProvider`
persists to `localStorage` and POSTs to `/api/density/preference`.

## Component library

Located under `portal/src/components/primitives/`:

| Component   | DOM emission                                  | Notes                                       |
|-------------|-----------------------------------------------|---------------------------------------------|
| `Button`    | `<button class="btn btn--{variant}">`         | Variants `primary / secondary / ghost / danger`, sizes `default / xs` |
| `Badge`     | `<span class="badge badge--{tone}">`          | Tones `ok / warn / err / ember / info / steel` |
| `Card`      | `<div class="card">`                          | Use with `<CardHeader>` and `<CardBody>` |
| `Kpi`       | `<div class="kpi">`                           | Number, unit, delta, sparkline, foot |
| `Chip`      | `<button class="chip" aria-pressed>`          | Filter chip with optional count |
| `Seg`       | `<div class="seg">`                           | Segmented control |
| `Sheet`     | Radix Dialog → `<div class="sheet">`          | Right-side drill-down panel |
| `Spark`     | Inline SVG sparkline                          | Used by `Kpi` |
| `PulseDot`  | `<span class="st st--{tone}">`                | Status dot |
| `Terminal`  | `<div class="terminal">`                      | Mono-terminal surface |
| `Code`      | `<pre class="code">`                          | Mono code surface |
| `ToastRail` | `<div class="toast-wrap">`                    | Mounted by `PortalShell` |

## Accessibility contract

- Every interactive control receives a visible `--ring` focus shadow on
  `:focus-visible`.
- Icon-only buttons must carry an `aria-label`.
- Colour is never the sole signal for state (status dots pair with text or
  icons).
- Modal Sheet uses Radix Dialog with focus trap and Escape close.
- Command palette announces result counts via an `aria-live="polite"` region.
- `@axe-core/playwright` runs against every page and asserts zero serious /
  critical violations in CI.

## Do / Don't

**Do**
- Use `var(--*)` tokens directly in inline `style` props for one-off layout.
- Use Tailwind utilities for layout (grid, flex, spacing) where the
  `tailwind.config.js` mapping resolves the token (e.g. `bg-bg-card`).
- Compose primitives into page-specific components — don't restyle the
  primitives.

**Don't**
- Don't hard-code colours, fonts, or shadows in component source.
- Don't import anything from `design/` — that folder ships the brand reference
  only.
- Don't write inline mocks; bind to real platform endpoints via the
  `portal/src/app/api/*` route handlers.

## Lattice / weave motif

The brand notebook uses a lattice/weave SVG pattern in chapter covers. The
Portal currently exposes it as the radial-gradient backdrop on the
`mesh-card`. For future surfaces that want the full weave, add the pattern
SVG to `portal/public/brand/weave.svg` and reference via `background-image:
url(/brand/weave.svg)` with a low opacity. Do NOT use CSS Houdini for this —
it doesn't render in Safari.
