# portal-command-palette Specification

## Purpose

The global ⌘K command palette of the Forge Portal — a `cmdk` + Radix Dialog
surface that aggregates results from a typed source registry (nav, agents,
skills, MCP, runs, approvals, specs, workspaces, tenants, actions), supports
action subcommands, and audits every invocation. Created by archiving change
`forge-portal-rebranding`.

## Requirements

### Requirement: Global keyboard shortcut

The Portal SHALL bind `Cmd+K` (macOS) and `Ctrl+K` (Windows / Linux / ChromeOS) at the document level to open a global command palette. The shortcut SHALL also be `/` when no other input is focused. The shortcut SHALL not fire inside `<input>`, `<textarea>` or `[contenteditable]` elements (unless that input is the top-bar search). Pressing `Escape` SHALL close the palette and return focus to the previously focused element.

#### Scenario: ⌘K opens the palette regardless of route

- **WHEN** the user presses `Cmd+K` on `/runtimes/abc123`
- **THEN** the palette dialog opens, focus moves to its search input, the page underneath is dimmed with `.scrim`, and `Escape` restores focus to the runtime card the user was previously on

#### Scenario: ⌘K inside a code editor does not trigger the palette

- **WHEN** the user is editing a workflow YAML in the editor (`<textarea>` focused) and presses `Cmd+K`
- **THEN** the palette does NOT open; the keystroke is delivered to the textarea (e.g. for line-jumping)

### Requirement: Source-pluggable command registry

The palette SHALL aggregate results from a typed source registry. The first release SHALL include the following sources, queried in parallel and ranked by score with a per-source cap:

- `nav`: every sidebar route plus deep links
- `agents`: assets from `GET /v1/registry/assets?kind=agent`
- `skills`: assets from `GET /v1/registry/assets?kind=skill`
- `mcp`: assets from `GET /v1/registry/assets?kind=mcp`
- `runs`: latest 200 runs from `GET /v1/sdlc/runs?limit=200`
- `approvals`: pending approvals from `GET /v1/approvals?status=pending&approver=<me>`
- `specs`: OpenSpecs from `GET /v1/openspec`
- `workspaces`: workspaces from `GET /v1/workspaces`
- `tenants`: tenants from `GET /v1/tenants/me`
- `actions`: theme/density/language toggles, "Toggle sidebar", "Sign out", "Switch workspace"

Each result SHALL render with: icon, title, optional subtitle (e.g. for runs the agent name + repo), a "source" eyebrow, and an optional kbd hint for direct actions. Results SHALL be navigable with arrow keys and selected with `Enter`.

#### Scenario: Typing "deploy" surfaces runs, approvals and the deploy route

- **WHEN** the user opens the palette and types `deploy`
- **THEN** the results panel shows at least: a `nav` row for `Deployments`, an `agents` row for `deploy-conductor`, an `approvals` row for any pending production deploy, and a `runs` row for any recent run with `purpose` containing "deploy"
- **AND** each row indicates its source via the eyebrow chip

#### Scenario: Selecting a run opens its detail sheet

- **WHEN** the palette is open and the user navigates to a `runs` row and presses `Enter`
- **THEN** the palette closes, the corresponding run sheet opens with steps populated from `GET /v1/sdlc/runs/{id}`, and the URL is updated to `?run={id}` for shareability

### Requirement: Action subcommands

The palette SHALL support inline action subcommands prefixed with `>`. Typing `>` SHALL show the action catalog (theme/density/lang/sidebar/sign-out/workspace). Typing `> tema` / `> theme` SHALL filter to theme actions.

#### Scenario: > theme dark switches the theme

- **WHEN** the user types `> theme dark` and presses `Enter`
- **THEN** the theme preference is set to `dark`, the toast fires, the palette closes, and the audit event `portal.command.invoked.v1` is emitted with `target_id: "action.theme.dark"`

### Requirement: Workspace and tenant switching

The palette SHALL support tenant/workspace switching as actions: typing `>` `tenant` or `>` `workspace` SHALL show the picker; selecting a workspace SHALL trigger the same flow as the sidebar tenant pill (persist via `POST /api/workspace/active`, audit, `router.refresh()`).

#### Scenario: > workspace switches active workspace

- **WHEN** the user picks `payments-platform` via the palette
- **THEN** the active workspace is updated, breadcrumb reflects the new workspace, and `router.refresh()` is invoked on the current route

### Requirement: Source results refresh and offline behaviour

The palette SHALL fetch sources fresh on open (no stale-while-revalidate). Each source request SHALL carry an `x-correlation-id` header and SHALL have a 1.5s timeout; sources that time out SHALL be marked as `(unreachable)` in the eyebrow chip and SHALL not block the rendering of other sources' results.

#### Scenario: One source is down — palette still works

- **WHEN** `/v1/sdlc/runs?limit=200` returns 503
- **THEN** the palette still shows `nav`, `agents`, `skills`, `mcp`, `approvals`, `specs`, `workspaces`, `tenants` and `actions` results; the `runs` group renders an inline "no disponible" / "unavailable" placeholder without breaking the UI

### Requirement: Accessibility for the palette

The palette SHALL be implemented on top of `cmdk` and `@radix-ui/react-dialog` and SHALL meet WCAG 2.1 AA: a labelled dialog role, focus trap, semantic `<ul role="listbox">` for results, and a visible focus ring on the search input. Screen readers SHALL announce the result count via an `aria-live="polite"` region after each keystroke.

#### Scenario: Result count is announced

- **WHEN** the user types `agen` and the palette returns 12 results
- **THEN** an `aria-live="polite"` region announces "12 resultados" / "12 results"

### Requirement: Telemetry for command usage

Every command invocation SHALL emit a `portal.command.invoked.v1` audit event with fields `principal`, `source`, `target_id` (route, asset id, run id or workspace id), `query` (free-text user input, max 200 chars, redacted of any tokens) and `correlation_id`. The events SHALL flow through the existing audit pipeline and SHALL be queryable in Grafana via the audit dashboard.

#### Scenario: Audit logs the command run

- **WHEN** the user invokes a theme action
- **THEN** the resulting audit event reads `{"type":"portal.command.invoked.v1","data":{"source":"actions","target_id":"action.theme.dark","query":"theme dark"}, ...}`
