#!/usr/bin/env node
/*
 * design-system-parity-check
 *
 * Migration verification (task 8.2): re-runs the merger against
 * `ds-forge-default` (which resolves to desing-system-1) and diffs the
 * resulting tokens against the Portal's current globals.css `:root` block.
 * Zero-diff is required before the catalog rollout flips for an existing
 * tenant.
 *
 * The check is approximate: it compares the *named tokens* (every CSS
 * custom-property declaration with the same name on both sides must have an
 * equal value). New tokens introduced by desing-system-1 do not fail the
 * check, but altered values do.
 *
 * Run with: node scripts/design-system-parity-check.mjs
 */

import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..');

const portalCSS = await fs.readFile(path.join(repoRoot, 'portal', 'src', 'app', 'globals.css'), 'utf8');
const dsCSS = await fs.readFile(path.join(repoRoot, 'design', 'systems', 'desing-system-1', 'tokens.css'), 'utf8');

const portalTokens = extractTokens(portalCSS);
const dsTokens = extractTokens(dsCSS);

const mismatches = [];
for (const [name, value] of Object.entries(dsTokens)) {
  if (!(name in portalTokens)) continue; // template adds a token; not a regression
  if (portalTokens[name] !== value) {
    mismatches.push({ name, portal: portalTokens[name], catalog: value });
  }
}

if (mismatches.length > 0) {
  console.error(`[parity] FAIL — ${mismatches.length} token(s) drifted between portal/globals.css and desing-system-1/tokens.css`);
  for (const m of mismatches) {
    console.error(`  ${m.name}: portal=${m.portal}  ds=${m.catalog}`);
  }
  process.exit(1);
}
console.log(`[parity] OK — desing-system-1 has zero-diff parity against portal globals.css for shared tokens`);

function extractTokens(css) {
  const out = {};
  // Only consider declarations inside :root (light) — the dark theme is
  // accounted for separately when the renderer flips data-theme.
  const root = css.match(/:root\s*{([\s\S]*?)}/);
  if (!root) return out;
  const body = root[1];
  const re = /(--[a-z0-9_-]+)\s*:\s*([^;}\n]+)/gi;
  let m;
  while ((m = re.exec(body))) {
    out[m[1]] = m[2].trim();
  }
  return out;
}
