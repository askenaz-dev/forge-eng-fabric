#!/usr/bin/env node
// Publishes the forge CLI to npm using the optionalDependencies-per-platform
// pattern. Reads goreleaser's dist/ directory and produces one prebuilt
// npm package per platform plus the parent shim package.
//
// Env:
//   VERSION   semver, may be prefixed with `v` (e.g. v0.2.0 -> 0.2.0). Required.
//   DIST      path to goreleaser dist directory. Default: ../dist relative to this file.
//   DRY_RUN   set to "1" to print actions without publishing.

import { mkdirSync, writeFileSync, copyFileSync, existsSync, readdirSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import os from 'node:os';

const here = path.dirname(fileURLToPath(import.meta.url));

const rawVersion = process.env.VERSION ?? '';
const version = rawVersion.replace(/^v/, '');
if (!version) {
  console.error('publish.mjs: VERSION env var required (e.g. v0.1.0)');
  process.exit(1);
}

const distRoot = path.resolve(process.env.DIST ?? path.join(here, '..', 'dist'));
const dryRun = process.env.DRY_RUN === '1';
const repoUrl = 'https://github.com/askenaz-dev/forge-eng-fabric';

const platforms = [
  { os: 'darwin', cpu: 'x64',   goos: 'darwin',  goarch: 'amd64', bin: 'forge' },
  { os: 'darwin', cpu: 'arm64', goos: 'darwin',  goarch: 'arm64', bin: 'forge' },
  { os: 'linux',  cpu: 'x64',   goos: 'linux',   goarch: 'amd64', bin: 'forge' },
  { os: 'linux',  cpu: 'arm64', goos: 'linux',   goarch: 'arm64', bin: 'forge' },
  { os: 'win32',  cpu: 'x64',   goos: 'windows', goarch: 'amd64', bin: 'forge.exe' },
];

function findBinary(p) {
  // goreleaser appends micro-arch suffixes that vary with version:
  // amd64 -> "_v1" (GOAMD64), arm64 -> "_v8.0" (GOARM64) in v2.x. Glob by prefix.
  const prefix = `forge_${p.goos}_${p.goarch}`;
  const matches = readdirSync(distRoot, { withFileTypes: true })
    .filter((e) => e.isDirectory() && (e.name === prefix || e.name.startsWith(prefix + '_')))
    .map((e) => e.name)
    .sort();
  for (const d of matches) {
    const candidate = path.join(distRoot, d, p.bin);
    if (existsSync(candidate)) return candidate;
  }
  throw new Error(
    `binary not found for ${p.goos}/${p.goarch} under ${distRoot} (looked for dirs matching ${prefix} or ${prefix}_*)`,
  );
}

function npmPublish(cwd) {
  if (dryRun) {
    console.log(`[dry-run] npm publish --access public  (cwd=${cwd})`);
    return;
  }
  execFileSync('npm', ['publish', '--access', 'public'], { cwd, stdio: 'inherit' });
}

const tmpRoot = path.join(os.tmpdir(), `forge-npm-${version}-${Date.now()}`);
mkdirSync(tmpRoot, { recursive: true });
console.log(`staging packages under ${tmpRoot}`);

const optionalDependencies = {};

for (const p of platforms) {
  const pkgName = `@askenaz-dev/forge-cli-${p.os}-${p.cpu}`;
  optionalDependencies[pkgName] = version;

  const dir = path.join(tmpRoot, pkgName);
  mkdirSync(path.join(dir, 'bin'), { recursive: true });
  copyFileSync(findBinary(p), path.join(dir, 'bin', p.bin));

  const manifest = {
    name: pkgName,
    version,
    description: `Forge CLI prebuilt binary for ${p.os}/${p.cpu}. Internal dependency of @askenaz-dev/forge-cli; do not install directly.`,
    repository: { type: 'git', url: `${repoUrl}.git`, directory: 'cli/forge' },
    license: 'Apache-2.0',
    os: [p.os],
    cpu: [p.cpu],
    bin: { forge: `bin/${p.bin}` },
  };
  writeFileSync(path.join(dir, 'package.json'), JSON.stringify(manifest, null, 2) + '\n');
  console.log(`publishing ${pkgName}@${version}`);
  npmPublish(dir);
}

const parentDir = path.join(tmpRoot, 'askenaz-dev-forge-cli');
mkdirSync(path.join(parentDir, 'bin'), { recursive: true });
copyFileSync(path.join(here, 'bin', 'forge.js'), path.join(parentDir, 'bin', 'forge.js'));

const readmeSrc = path.join(here, 'README.md');
if (existsSync(readmeSrc)) {
  copyFileSync(readmeSrc, path.join(parentDir, 'README.md'));
}

const parentManifest = {
  name: '@askenaz-dev/forge-cli',
  version,
  description: 'Forge developer CLI — install governed Forge skills into your agent (Claude Code, Copilot, Codex, Cursor, Gemini CLI, …).',
  repository: { type: 'git', url: `${repoUrl}.git`, directory: 'cli/forge' },
  license: 'Apache-2.0',
  bin: { forge: 'bin/forge.js' },
  files: ['bin/'],
  optionalDependencies,
  engines: { node: '>=18' },
};
writeFileSync(path.join(parentDir, 'package.json'), JSON.stringify(parentManifest, null, 2) + '\n');
console.log(`publishing @askenaz-dev/forge-cli@${version}`);
npmPublish(parentDir);

console.log('done.');
