# Skill gateway — install matrix

The `forge` CLI auto-detects the developer's active client and writes the skill bundle into the canonical directory. The same bundle works in every client because we use the open Agent Skills format unmodified.

## Supported clients

| Client | ID | Skills directory | MCP config |
|---|---|---|---|
| Claude Code | `claude-code` | `~/.claude/skills/<name>/` | `~/.claude/mcp.json` |
| Claude desktop | `claude-desktop` | `~/Library/Application Support/Claude/skills/<name>/` (macOS) / `%APPDATA%\Claude\skills\<name>\` (Windows) | `claude_desktop_config.json` |
| GitHub Copilot | `copilot` | `~/.config/github-copilot/skills/<name>/` | `~/.config/github-copilot/mcp.json` |
| OpenAI Codex CLI | `codex` | `~/.codex/skills/<name>/` | `~/.codex/mcp.json` |
| Cursor | `cursor` | `~/.cursor/skills/<name>/` | `~/.cursor/mcp.json` |
| Gemini CLI | `gemini-cli` | `~/.gemini/skills/<name>/` | `~/.gemini/mcp.json` |
| OpenHands | `openhands` | `~/.openhands/skills/<name>/` | `~/.openhands/mcp.json` |
| OpenCode | `opencode` | `~/.opencode/skills/<name>/` | `~/.opencode/mcp.json` |
| VS Code (workspace) | `vscode` | `<repo>/.vscode/skills/<name>/` | `<repo>/.vscode/mcp.json` |
| Generic / unknown | `generic` | `~/.agentskills/<name>/` | — |

The table above is the source of truth for the CLI as well. Adding a new client is a CLI release that updates `cli/forge/internal/clients/clients.go`.

## Day-1 install

Pick the channel that matches your environment. Both ship the same signed binary, produced by [`.github/workflows/cli-release.yml`](../../.github/workflows/cli-release.yml) on every `v*` tag.

### Homebrew (macOS / Linux)

```bash
brew install askenaz-dev/tap/forge
```

Backed by the [`askenaz-dev/homebrew-tap`](https://github.com/askenaz-dev/homebrew-tap) repo, which goreleaser updates as part of the release job.

### npm (any OS with Node ≥ 18)

```bash
npm install -g @askenaz-dev/forge-cli
# or: pnpm add -g @askenaz-dev/forge-cli   /   yarn global add @askenaz-dev/forge-cli   /   bun add -g @askenaz-dev/forge-cli
```

`@askenaz-dev/forge-cli` uses platform-specific `optionalDependencies` (`@askenaz-dev/forge-cli-darwin-arm64`, `@askenaz-dev/forge-cli-linux-x64`, `@askenaz-dev/forge-cli-win32-x64`, …): your package manager installs only the one binary that matches your OS/CPU. No postinstall scripts, no network calls outside the registry — works in air-gapped/`--ignore-scripts` setups.

Supported targets: `darwin/x64`, `darwin/arm64`, `linux/x64`, `linux/arm64`, `win32/x64`. On unsupported platforms, fall back to:

```bash
go install github.com/forge-eng-fabric/cli/forge/cmd/forge@latest
```

### Direct download

Archives + `checksums.txt` are attached to every [GitHub release](https://github.com/askenaz-dev/forge-eng-fabric/releases).

### First-run

```bash
forge login --gateway https://<tenant>.forge.dev
forge skills list
forge skills install generate-test-cases
```

`forge skills install` resolves the active client automatically. To override:

```bash
forge skills install foo --client codex
```

The CLI verifies the bundle's sha256 against the gateway's `X-Forge-Package-Digest` before extracting; a mismatch aborts the install and leaves the client directory untouched.

## MCP wiring

If the installed skill declares `mcp:` dependencies in its front-matter, the CLI inserts the corresponding remote MCP endpoint into the active client's MCP config under a `forge:` namespace. The entry is idempotent — re-running install produces zero diffs.

Example for Claude Code (`~/.claude/mcp.json`):

```json
{
  "servers": {
    "forge:github": {
      "transport": "http",
      "url": "https://acme.forge.dev/v1/gateway/mcp/<github-asset-id>",
      "headers": { "Authorization": "Bearer ${FORGE_TOKEN}" },
      "forge_asset_id": "<github-asset-id>"
    }
  }
}
```

Claude desktop expects the same shape under `mcpServers` instead of `servers`; the CLI handles the difference.

## Updating and uninstalling

```bash
forge skills update                     # all installed skills
forge skills update generate-test-cases # one
forge skills remove generate-test-cases
forge skills status                     # per-client inventory
```

`remove` also prunes the `forge:` MCP entry if no other skill needs it. Other entries in the file (anything not under the `forge:` namespace) are left alone.

## CI usage

The CLI accepts `FORGE_TOKEN=<pat>` from the environment, so CI jobs do not need a browser flow:

```bash
export FORGE_TOKEN=$FORGE_PAT
forge skills install scaffold-service --client generic
```

## Release-pipeline setup (one-time, for platform owners)

Before the first `v*` tag works end-to-end, the following must exist:

1. **GitHub org `askenaz-dev`** with a public repo named exactly `homebrew-tap` (empty is fine — goreleaser commits the formula on the first release).
2. **Secrets on `askenaz-dev/forge-eng-fabric`**:
   - `HOMEBREW_TAP_GITHUB_TOKEN` — PAT with `repo` scope on `askenaz-dev/homebrew-tap`.
   - `NPM_TOKEN` — automation token for the npm account/org that owns the `@askenaz-dev` scope.
3. **npm scope `@askenaz-dev`** owned by your npm account or org (one-time, locks the namespace).
4. Tag the commit you want to ship: `git tag v0.1.0 && git push origin v0.1.0`. The [`cli-release`](../../.github/workflows/cli-release.yml) workflow runs goreleaser + npm publish.

Local dry-run without publishing:

```bash
cd cli/forge
goreleaser release --snapshot --clean      # builds dist/ for all targets
cd npm && VERSION=v0.0.0-dev DIST=../dist DRY_RUN=1 node publish.mjs
```
