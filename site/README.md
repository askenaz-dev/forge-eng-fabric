# Forge marketing site

This folder is published to GitHub Pages as the public marketing landing for
the Forge Engineering Fabric platform.

## What it is

A single static HTML page (`index.html`) plus a stylesheet (`assets/forge.css`)
and a small interaction script (`assets/forge.js`). No build step, no Jekyll,
no external CDN dependencies beyond Google Fonts (Instrument Serif + Geist +
JetBrains Mono are imported via the standard `<link rel="stylesheet">`).

The visual language matches the Portal design system — same Ember palette,
same Instrument Serif display, same Geist UI body, same JetBrains Mono code.
Light theme is default; dark theme follows `prefers-color-scheme` and is
togglable via the navbar.

## Local preview

Open `site/index.html` in any modern browser — there is no build step.

For a smoother dev loop (live reload on edit), run any static server:

```sh
cd site
python -m http.server 4000
# → http://localhost:4000
```

## Deploy

`.github/workflows/gh-pages.yml` handles publication:

1. Trigger: push to `main` that touches `site/**`, or manual `workflow_dispatch`.
2. The workflow uploads `site/` as a Pages artefact and deploys via
   `actions/deploy-pages@v4`.
3. **One-time setup**: in **Settings → Pages → Source**, choose **GitHub
   Actions** (not "Deploy from a branch"). The first run of the workflow will
   provision the Pages site automatically.

## File layout

```
site/
├─ index.html          # the page
├─ .nojekyll           # tells GH Pages to skip Jekyll
├─ assets/
│  ├─ forge.css        # tokens + components (mirrors the Portal DS)
│  ├─ forge.js         # theme toggle, smooth scroll, ⌘K affordance
│  └─ favicon.svg      # the Forge mark
└─ README.md           # this file
```

## Editing

- Copy lives directly in `index.html` — it's a single page, so search-and-replace
  is the fastest editing path.
- The design tokens are at the top of `forge.css`. Edits there cascade to the
  whole page.
- If a structural change is needed, prefer creating a new section that reuses
  the existing component classes (`.chapter`, `.two-col`, `.pillars`, `.stats`,
  `.flow`, `.arch`, `.cta-band`).

## Conventions

- All visible strings are in English. (The Portal is bilingual ES/EN; the
  marketing site is intentionally single-locale to keep copy tight. A future
  variant can fork this folder.)
- No external JS frameworks. Keep `forge.js` under ~3 KB.
- No analytics scripts ship without explicit privacy review.
- Images that would otherwise be raster (logos, marks) ship as inline SVG or
  `.svg` files — keeps the site portable.
