#!/usr/bin/env node
'use strict';

const { spawnSync } = require('node:child_process');
const { existsSync } = require('node:fs');
const path = require('node:path');

const platform = process.platform;
const arch = process.arch;
const subpkg = `@askenaz-dev/forge-cli-${platform}-${arch}`;

let binPath;
try {
  const pkgJsonPath = require.resolve(`${subpkg}/package.json`);
  const binName = platform === 'win32' ? 'forge.exe' : 'forge';
  binPath = path.join(path.dirname(pkgJsonPath), 'bin', binName);
} catch {
  console.error(`forge: no prebuilt binary for ${platform}/${arch}.`);
  console.error('Supported: darwin/x64, darwin/arm64, linux/x64, linux/arm64, win32/x64.');
  console.error('Alternative: go install github.com/forge-eng-fabric/cli/forge/cmd/forge@latest');
  process.exit(1);
}

if (!existsSync(binPath)) {
  console.error(`forge: optional dependency installed but binary missing at ${binPath}`);
  console.error('Try: npm install --force @askenaz-dev/forge-cli');
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), { stdio: 'inherit' });
if (result.error) {
  console.error('forge:', result.error.message);
  process.exit(1);
}
process.exit(result.status ?? 1);
