# @askenaz-dev/forge-cli (npm distribution)

This directory builds and publishes the `forge` CLI to npm under the `@askenaz-dev` scope using the
[optionalDependencies-per-platform](https://esbuild.github.io/getting-started/#download-a-build) pattern (same approach used by `esbuild`, `biome`, `swc`).

## Layout

- `bin/forge.js` — committed JS shim shipped inside the `@askenaz-dev/forge-cli` parent package; resolves and execs the platform-specific binary.
- `publish.mjs` — release-time orchestrator. Reads `cli/forge/dist/` (produced by goreleaser), generates one prebuilt npm package per platform, then publishes the parent.

## How it works

Installing `@askenaz-dev/forge-cli` lets npm/pnpm/yarn/bun pick the matching `@askenaz-dev/forge-cli-<platform>-<arch>` optional dependency (skipping the others). The parent's `bin/forge.js` then `spawnSync`'s the binary from that sub-package.

```
@askenaz-dev/forge-cli@X.Y.Z
├─ optionalDependencies
│   ├─ @askenaz-dev/forge-cli-darwin-x64@X.Y.Z
│   ├─ @askenaz-dev/forge-cli-darwin-arm64@X.Y.Z
│   ├─ @askenaz-dev/forge-cli-linux-x64@X.Y.Z
│   ├─ @askenaz-dev/forge-cli-linux-arm64@X.Y.Z
│   └─ @askenaz-dev/forge-cli-win32-x64@X.Y.Z
└─ bin/forge.js  → resolves & execs the matching sub-package's binary
```

## Release flow

1. Tag the repo with `vX.Y.Z`.
2. [`.github/workflows/cli-release.yml`](../../../.github/workflows/cli-release.yml) runs goreleaser → produces `cli/forge/dist/`, the GitHub release, and the Homebrew formula in `askenaz-dev/homebrew-tap`.
3. The same workflow runs `node cli/forge/npm/publish.mjs` with `VERSION=$GITHUB_REF_NAME` to publish all 5 platform packages + the parent.

## Local dry-run

```bash
cd cli/forge
goreleaser release --snapshot --clean
cd npm
VERSION=v0.0.0-dev DIST=../dist DRY_RUN=1 node publish.mjs
```

## Secrets required in CI

| Secret | Purpose |
|---|---|
| `NPM_TOKEN` | Automation token for the npm account/org that owns the `@askenaz-dev` scope |
| `HOMEBREW_TAP_GITHUB_TOKEN` | PAT with `repo` scope on `askenaz-dev/homebrew-tap` |
