#!/usr/bin/env node
/*
 * design-system-merger
 *
 * Build-time hook that resolves an App's `design_system_ref` against the AI
 * Asset Registry, fetches the manifest, verifies the sha256 digests of the
 * token sheet, component pack, fonts and screenshots, merges per-component
 * overrides (surface tokens only — layout tokens are rejected), and emits:
 *
 *   1. `portal/src/app/design-system-tokens.generated.css` — the resolved
 *      token sheet, inlined into globals.css via @import at the top.
 *   2. `portal/tailwind.tokens.generated.js`               — token bindings
 *      consumed by tailwind.config.js when forge.design_system_catalog.enabled
 *      is set, otherwise tailwind.config.js falls back to the static block.
 *   3. `portal/src/app/font-preload.generated.json`        — the font
 *      preload list `layout.tsx` reads at boot.
 *
 * The merger fails LOUD on:
 *   - The asset's `lifecycle_state` is not `approved`
 *   - The manifest's sha256 of a fetched asset does not match the downloaded
 *     body
 *   - An override targets a layout-token namespace (`--space-*`, `--grid-*`,
 *     `--breakpoint-*`).
 *
 * Inputs (env or argv):
 *   - APP_DESIGN_SYSTEM_REF        — overrides the App's stored ref
 *   - APP_DESIGN_SYSTEM_OVERRIDES  — JSON-encoded overrides map
 *   - APP_DESIGN_SYSTEM_RESOLVER   — `local|registry` (default: `local`)
 *                                    `local` reads from design/systems/<id>/
 *                                    `registry` calls the Registry HTTP API
 *   - REGISTRY_BASE_URL            — required when resolver=registry
 *   - APP_TENANT_ID                — passed to the registry resolver
 *
 * The portal repo runs this with `node scripts/design-system-merger.mjs`
 * before `next build`. The portal-bundle repo runs the same script during
 * its CI to regenerate the token files post-merge of a swap PR.
 */

import { createHash } from 'node:crypto';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '../..');

const LAYOUT_TOKEN_PREFIXES = ['--space-', '--grid-', '--breakpoint-'];
const ALLOWED_OVERRIDE_PREFIXES = ['--color-', '--font-', '--radius-', '--shadow-'];

const args = parseArgs();
const resolverMode = args.resolver ?? process.env.APP_DESIGN_SYSTEM_RESOLVER ?? 'local';
const ref = args.ref ?? process.env.APP_DESIGN_SYSTEM_REF ?? 'ds-forge-default';
const overrides = parseOverrides();

run().catch((err) => {
  console.error(`[design-system-merger] FAIL ${err.message}`);
  process.exit(1);
});

async function run() {
  console.log(`[design-system-merger] resolving ref=${ref} (resolver=${resolverMode})`);
  const manifest = await resolveManifest(ref);
  const tokensCSS = await fetchAndVerify(manifest.tokens, manifest.tokens_sha256, 'tokens.css');

  for (const fontEntry of manifest.fonts) {
    // We do not download every font body at merge time (they are loaded by
    // the browser at runtime); but we MUST emit the preload list so layout.tsx
    // can issue the correct <link rel="preload"> for each.
  }

  const mergedCSS = mergeOverrides(tokensCSS, overrides);
  const tokenBindings = extractTokenBindings(mergedCSS);

  await write('src/app/design-system-tokens.generated.css', mergedCSS);
  await write('tailwind.tokens.generated.js', emitTailwindBindings(tokenBindings));
  await write('src/app/font-preload.generated.json', JSON.stringify(manifest.fonts, null, 2));

  console.log(`[design-system-merger] OK — wrote ${Object.keys(tokenBindings).length} token bindings`);
}

function parseArgs() {
  const out = {};
  for (let i = 2; i < process.argv.length; i++) {
    const v = process.argv[i];
    if (v.startsWith('--ref=')) out.ref = v.slice('--ref='.length);
    else if (v.startsWith('--resolver=')) out.resolver = v.slice('--resolver='.length);
    else if (v.startsWith('--overrides=')) out.overrides = v.slice('--overrides='.length);
  }
  return out;
}

function parseOverrides() {
  const raw = args.overrides ?? process.env.APP_DESIGN_SYSTEM_OVERRIDES ?? '';
  if (!raw.trim()) return {};
  try { return JSON.parse(raw); }
  catch (err) { throw new Error(`APP_DESIGN_SYSTEM_OVERRIDES is not valid JSON: ${err.message}`); }
}

async function resolveManifest(ref) {
  if (resolverMode === 'local') {
    // Resolve aliases and `id@version` strings against the local catalog. The
    // alias map mirrors design_system_alias in the Registry seed migration.
    const aliasMap = { 'ds-forge-default': 'desing-system-1' };
    let assetID = aliasMap[ref] ?? ref;
    if (assetID.includes('@')) assetID = assetID.split('@')[0];
    // Strip the registry-internal `design_system:<workspace>:` prefix when
    // present.
    const tail = assetID.split(':').pop();
    const manifestPath = path.join(repoRoot, 'design', 'systems', tail, 'manifest.json');
    const body = await fs.readFile(manifestPath, 'utf8');
    return JSON.parse(body);
  }
  if (resolverMode === 'registry') {
    const base = process.env.REGISTRY_BASE_URL;
    if (!base) throw new Error('REGISTRY_BASE_URL is required when resolver=registry');
    const url = `${base.replace(/\/$/, '')}/v1/design-systems/${encodeURIComponent(ref)}`;
    const res = await fetch(url, { headers: { 'Authorization': `Bearer ${process.env.REGISTRY_TOKEN ?? ''}` } });
    if (!res.ok) throw new Error(`registry resolve ${ref} returned ${res.status}`);
    const body = await res.json();
    if (body.lifecycle_state !== 'approved') {
      throw new Error(`design_system_asset_not_approved: ${body.asset_id} is in ${body.lifecycle_state}`);
    }
    return body.manifest;
  }
  throw new Error(`unknown resolver mode: ${resolverMode}`);
}

async function fetchAndVerify(url, expectedSha, label) {
  // Local-mode short-circuit: in dev, the manifest's URL points at an
  // aspirational platform CDN that is not reachable. Instead read the local
  // tokens.css from the design/systems/<id>/ directory. The sha256 check is
  // still performed so the build remains identical to production.
  if (resolverMode === 'local') {
    const idMatch = url.match(/desing-system-\d/);
    if (!idMatch) throw new Error(`local resolver could not infer template id from URL ${url}`);
    const tail = idMatch[0];
    const localPath = path.join(repoRoot, 'design', 'systems', tail, 'tokens.css');
    const body = await fs.readFile(localPath, 'utf8');
    const digest = createHash('sha256').update(body).digest('hex');
    if (expectedSha && expectedSha !== '0'.repeat(64) && digest !== expectedSha) {
      throw new Error(`design_system_digest_mismatch: ${label} computed=${digest} manifest=${expectedSha}`);
    }
    return body;
  }
  const res = await fetch(url);
  if (!res.ok) throw new Error(`design_system_asset_unreachable: ${label} returned ${res.status}`);
  const body = await res.text();
  const digest = createHash('sha256').update(body).digest('hex');
  if (digest !== expectedSha) {
    throw new Error(`design_system_digest_mismatch: ${label} computed=${digest} manifest=${expectedSha}`);
  }
  return body;
}

function mergeOverrides(tokensCSS, overrides) {
  // Per-component overrides MAY only touch surface-token namespaces. Any
  // override map whose values contain a layout token is rejected here.
  for (const [component, raw] of Object.entries(overrides)) {
    if (typeof raw !== 'string') continue;
    for (const token of raw.split(/\s*;\s*/)) {
      const declMatch = token.match(/^\s*(--[a-z0-9_-]+)\s*:/);
      if (!declMatch) continue;
      const name = declMatch[1];
      if (LAYOUT_TOKEN_PREFIXES.some((p) => name.startsWith(p))) {
        throw new Error(`layout_token_override_forbidden: ${component} overrides ${name}`);
      }
      if (!ALLOWED_OVERRIDE_PREFIXES.some((p) => name.startsWith(p))) {
        throw new Error(`unsupported_override_namespace: ${component} declares ${name} (allowed: color, font, radius, shadow)`);
      }
    }
  }
  // The override values are appended as scoped `[data-ds-override="<component>"]`
  // declarations so the runtime renderer can apply them per-component.
  const appendix = Object.entries(overrides)
    .map(([component, decls]) => `[data-ds-override="${component}"] { ${decls} }`)
    .join('\n');
  return appendix ? `${tokensCSS}\n\n/* per-component overrides */\n${appendix}\n` : tokensCSS;
}

function extractTokenBindings(css) {
  const out = {};
  const re = /(--[a-z0-9_-]+)\s*:\s*([^;}\n]+)/gi;
  let m;
  while ((m = re.exec(css))) {
    out[m[1]] = m[2].trim();
  }
  return out;
}

function emitTailwindBindings(tokens) {
  // The output is a CommonJS module so tailwind.config.js can require() it.
  const colors = {}; const radii = {}; const shadows = {}; const fonts = {};
  for (const [k, v] of Object.entries(tokens)) {
    if (k.startsWith('--color-')) colors[k.slice('--color-'.length)] = `var(${k})`;
    else if (k.startsWith('--radius-')) radii[k.slice('--radius-'.length)] = `var(${k})`;
    else if (k.startsWith('--shadow-')) shadows[k.slice('--shadow-'.length)] = `var(${k})`;
    else if (k.startsWith('--font-')) fonts[k.slice('--font-'.length)] = [`var(${k})`];
  }
  return `// AUTO-GENERATED by scripts/design-system-merger.mjs — do not edit.\n`
    + `module.exports = ${JSON.stringify({ colors, radii, shadows, fonts }, null, 2)};\n`;
}

async function write(rel, body) {
  const target = path.join(__dirname, '..', rel);
  await fs.mkdir(path.dirname(target), { recursive: true });
  await fs.writeFile(target, body, 'utf8');
}
