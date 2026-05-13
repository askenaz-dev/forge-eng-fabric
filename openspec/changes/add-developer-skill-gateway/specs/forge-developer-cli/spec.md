## ADDED Requirements

### Requirement: Single-binary CLI

The platform SHALL ship a single static `forge` binary for macOS (arm64, amd64), Linux (arm64, amd64) and Windows (amd64), distributed via signed release artifacts and a homebrew tap / scoop bucket. The binary SHALL not require Node, Python or Docker at runtime.

#### Scenario: Install with no dependencies

- **WHEN** a developer downloads the release archive for their platform and runs `forge --version`
- **THEN** the command prints a version without prompting for additional installs

### Requirement: Authentication subcommand

`forge login` SHALL run an OIDC device-code flow against a gateway URL, store the resulting refresh token in the OS-native secret store (Keychain on macOS, Credential Manager on Windows, libsecret/keyring on Linux) and never write tokens to plain files in the home directory. `forge logout` SHALL remove the entry.

#### Scenario: Login stores token in OS keystore

- **WHEN** the developer completes `forge login --gateway https://gw.forge.acme.io`
- **THEN** the refresh token is written under service `forge` in the OS keystore
- **AND** no plain-text token file is created under `$HOME/.forge`

#### Scenario: PAT alternative

- **WHEN** the developer sets `FORGE_TOKEN=<pat>` in the environment
- **THEN** the CLI uses the PAT for all calls and skips OIDC

### Requirement: List and search

`forge skills list` SHALL print installable assets returned by the gateway, scoped to the current Tenant + Workspace, with columns `id`, `version`, `type`, `trust_level`, `installed?`. `forge skills search <query>` SHALL perform server-side substring / tag search.

#### Scenario: List shows install state per client

- **WHEN** the developer runs `forge skills list`
- **THEN** items already installed on the active client are marked `installed`
- **AND** items installed on a different detected client are marked `installed (on <client>)`

### Requirement: Install resolves the correct client directory

`forge skills install <name>[@<version>]` SHALL detect the active agentic client (or accept `--client <name>`) and write the unpacked Agent Skills bundle into the canonical directory of that client:

| Client | Path |
|---|---|
| Claude Code | `~/.claude/skills/<name>/` |
| Claude desktop | `~/Library/Application Support/Claude/skills/<name>/` (macOS) and platform equivalents |
| GitHub Copilot (CLI / VS Code) | `~/.config/github-copilot/skills/<name>/` |
| OpenAI Codex CLI | `~/.codex/skills/<name>/` |
| Cursor | `~/.cursor/skills/<name>/` |
| Gemini CLI | `~/.gemini/skills/<name>/` |
| OpenHands | `~/.openhands/skills/<name>/` |
| OpenCode | `~/.opencode/skills/<name>/` |
| VS Code (workspace) | `<repo>/.vscode/skills/<name>/` |
| Generic / unknown | `~/.agentskills/<name>/` |

The CLI SHALL refuse to write outside the resolved directory and SHALL verify the bundle digest + signature before extraction.

#### Scenario: Auto-detect Claude Code

- **GIVEN** `~/.claude/` exists on the developer's machine
- **WHEN** `forge skills install generate-test-cases` is run with no `--client`
- **THEN** the bundle is extracted to `~/.claude/skills/generate-test-cases/`
- **AND** the CLI prints the resolved client and path

#### Scenario: Explicit client overrides detection

- **WHEN** the developer runs `forge skills install foo --client codex`
- **THEN** the bundle is installed under `~/.codex/skills/foo/` regardless of other detected clients

#### Scenario: Digest mismatch aborts install

- **WHEN** the downloaded bundle's sha256 differs from the gateway-advertised digest
- **THEN** the CLI deletes the temp file, prints the mismatch and exits non-zero
- **AND** no files are written to the client directory

### Requirement: MCP registration on install

When an installed skill bundle declares one or more MCP dependencies in `SKILL.md` front-matter, `forge skills install` SHALL append the corresponding remote MCP endpoint (gateway URL + asset id) to the active client's MCP configuration file (e.g., `~/.claude/mcp.json`, `.vscode/mcp.json`, `claude_desktop_config.json`) under a `forge:` namespace, idempotently.

#### Scenario: MCP added once

- **WHEN** a skill declaring `mcp: ["github"]` is installed for Claude Code
- **THEN** `~/.claude/mcp.json` gains an entry `forge:github` pointing to `https://gw.forge.acme.io/v1/gateway/mcp/<github-asset-id>` with a `Bearer ${FORGE_TOKEN}` header reference
- **AND** running the install a second time produces zero diffs in the file

### Requirement: Update, remove, status

`forge skills update [<name>]` SHALL upgrade installed skills to their latest approved version (all skills when no name is given). `forge skills remove <name>` SHALL delete the installed directory and prune the MCP entry. `forge skills status` SHALL print, per skill, the installed version, the latest available version, drift indicators and the source gateway.

#### Scenario: Update is idempotent at latest

- **GIVEN** a skill installed at its latest version
- **WHEN** `forge skills update <name>` runs
- **THEN** the CLI prints `up-to-date` and exits zero without writing files

#### Scenario: Remove prunes MCP entry

- **WHEN** `forge skills remove generate-test-cases` runs
- **THEN** the skill directory is deleted
- **AND** any MCP entry under the `forge:` namespace that only referenced this skill is removed
- **AND** the developer's MCP entries outside `forge:` are untouched

### Requirement: Telemetry opt-out

The CLI SHALL emit anonymous, aggregated install telemetry to the gateway by default and SHALL respect `FORGE_NO_TELEMETRY=1` and `forge config set telemetry off`. No source file paths or repository contents SHALL leave the developer's machine via telemetry.

#### Scenario: Telemetry disabled

- **GIVEN** `FORGE_NO_TELEMETRY=1`
- **WHEN** any CLI command runs
- **THEN** no telemetry HTTP request is issued
- **AND** the command output prints `telemetry: off`
