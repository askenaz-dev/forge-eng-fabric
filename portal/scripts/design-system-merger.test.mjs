#!/usr/bin/env node
/*
 * Snapshot smoke-tests for the design-system-merger. The full pixel-diff
 * suite lives in playwright (see playwright/design-system.spec.ts); this
 * harness only checks the merger's output deterministically by:
 *   1. Resolving each of the four built-in templates via the local resolver
 *   2. Asserting the merger emits non-empty tokens, tailwind bindings and a
 *      font preload list
 *   3. Verifying the rejected-override invariant (layout-token overrides
 *      crash the build).
 *
 * Run with: node scripts/design-system-merger.test.mjs
 */

import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { promises as fs } from 'node:fs';

const execAsync = promisify(exec);
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const templates = ['desing-system-1', 'desing-system-2', 'desing-system-3', 'desing-system-4'];
let failures = 0;

for (const id of templates) {
  await runOk(`${id} via local resolver`, async () => {
    await execAsync(`node ${path.join(__dirname, 'design-system-merger.mjs')} --resolver=local --ref=${id}`);
    const tokens = await fs.readFile(path.join(__dirname, '..', 'src', 'app', 'design-system-tokens.generated.css'), 'utf8');
    if (!tokens.includes('--color-primary')) throw new Error('tokens missing --color-primary');
    const bindings = await fs.readFile(path.join(__dirname, '..', 'tailwind.tokens.generated.js'), 'utf8');
    if (!bindings.includes('colors')) throw new Error('tailwind bindings missing colors block');
    const fonts = JSON.parse(await fs.readFile(path.join(__dirname, '..', 'src', 'app', 'font-preload.generated.json'), 'utf8'));
    if (!Array.isArray(fonts) || fonts.length === 0) throw new Error('font preload list empty');
  });
}

await runFail('layout-token override rejected', async () => {
  await execAsync(`node ${path.join(__dirname, 'design-system-merger.mjs')} --resolver=local --ref=desing-system-1`, {
    env: { ...process.env, APP_DESIGN_SYSTEM_OVERRIDES: JSON.stringify({ card: '--space-1: 99px' }) },
  });
}, /layout_token_override_forbidden/);

process.exit(failures > 0 ? 1 : 0);

async function runOk(name, fn) {
  try {
    await fn();
    console.log(`✓ ${name}`);
  } catch (err) {
    console.error(`✗ ${name}: ${err.message}`);
    failures++;
  }
}

async function runFail(name, fn, expectedPattern) {
  try {
    await fn();
    console.error(`✗ ${name}: expected failure but command succeeded`);
    failures++;
  } catch (err) {
    const message = err.stderr ?? err.message;
    if (expectedPattern.test(message)) {
      console.log(`✓ ${name}`);
    } else {
      console.error(`✗ ${name}: expected ${expectedPattern} but got ${message}`);
      failures++;
    }
  }
}
