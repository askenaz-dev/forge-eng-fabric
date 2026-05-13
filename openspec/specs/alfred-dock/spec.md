# alfred-dock Specification

## Purpose

Portal-wide bottom-right floating launcher and slide-in console for invoking
Alfred from any authenticated route, showing live agent-mode session state,
accepting follow-up intents, and routing the user to the underlying OpenSpec
/ PR / deploy URL. Created by archiving change
`alfred-agent-mode-orchestrator`.

## Requirements

### Requirement: Bottom-right launcher on every authenticated route

The Portal SHALL render a persistent **Alfred launcher** anchored to the bottom-right of the viewport on every authenticated route. The launcher SHALL render above the toast rail (`z-index` ≥ toast layer + 1) and below modal overlays. It SHALL show the Alfred mark, the current session's status badge when a session is active, and a count of unread session events when the panel is closed.

#### Scenario: Launcher appears on every authenticated route

- **WHEN** an authenticated user with `alfred:invoke` navigates to `/`, `/agents`, `/runs`, or any other authenticated route
- **THEN** a `<button class="alfred-dock-launcher">` is rendered at `position: fixed; right: var(--spc-4); bottom: var(--spc-4)` containing the Alfred mark
- **AND** on routes where the user lacks `alfred:invoke` or the workspace flag `alfred.dock_enabled` is `false`, the launcher SHALL NOT render

#### Scenario: Launcher reflects active session status

- **WHEN** an agent-mode session for the active workspace is `running`
- **THEN** the launcher SHALL display the `working` variant of the mark and a status badge with the localized text `Working…` / `Trabajando…`
- **AND** when a session is `paused_for_approval` the launcher SHALL display the `attention` variant and a pulse animation until the user opens the panel

### Requirement: Slide-in console with plan and live transcript

Clicking the launcher SHALL open a slide-in console panel (anchored bottom-right, default `width: 420px`, `height: min(720px, 80vh)`) containing: a header with the session title and status, the current plan as an ordered checklist with per-step status icons, a live transcript of decisions and tool calls, a follow-up intent input, and quick links to the OpenSpec, PR and deploy URL when present.

#### Scenario: Opening the panel reveals plan and live transcript

- **WHEN** the user clicks the launcher while an active session exists
- **THEN** the panel SHALL slide in from the right, set focus to the follow-up input, render the plan with one row per step (status icon, label, optional approver name when waiting), and start consuming the session SSE stream
- **AND** new transcript entries SHALL append at the bottom and auto-scroll only when the user is already at the bottom

#### Scenario: Opening with no active session shows an empty composer

- **WHEN** the user clicks the launcher and no session is active in the workspace
- **THEN** the panel SHALL show an empty composer with prompts (`Describe what you want to ship` / `Describe lo que quieres entregar`), a workspace selector defaulting to the active workspace, and a `Start agent-mode` primary button gated by `alfred:agent-mode.run`

### Requirement: Keyboard summon and focus trap

The launcher SHALL be summonable with a global keyboard shortcut and the open panel SHALL be a focus trap until dismissed.

#### Scenario: Hotkey opens the dock

- **WHEN** the user presses `Alt+A` (Windows/Linux) or `⌥A` (macOS)
- **THEN** the dock panel SHALL toggle open or close, regardless of which route is focused, except when a text input or command palette is already capturing the keystroke

#### Scenario: Focus is trapped inside the open dock

- **WHEN** the dock is open and the user presses `Tab` repeatedly
- **THEN** focus SHALL cycle only through interactive elements inside the panel; pressing `Esc` SHALL close the panel and restore focus to the launcher button

### Requirement: Responsive and shell-coexistence rules

The dock SHALL coexist with the existing portal shell rules: collapsed sidebar, command palette, toast rail, notifications popover.

#### Scenario: Below 1024px the dock collapses to icon-only

- **WHEN** the viewport is narrower than 1024px
- **THEN** the open panel SHALL widen to `min(100vw - 32px, 480px)` and the launcher SHALL hide its status badge text (icon-only)
- **AND** the panel SHALL refuse to render concurrently with the command palette — opening one closes the other

#### Scenario: Toast rail stacks below the dock

- **WHEN** a toast appears while the dock is open
- **THEN** the toast SHALL be offset upward by the dock's height so it remains visible

### Requirement: Permission and workspace flag gating

The dock SHALL respect the same permission and workspace-flag gates as the underlying agent-mode capability. Starting a session from the dock SHALL surface policy errors inline without dismissing the panel.

#### Scenario: User without agent-mode permission can read but not start

- **WHEN** the principal has `alfred:invoke` but lacks `alfred:agent-mode.run`
- **THEN** the dock SHALL render and stream sessions started by others in the workspace, but the `Start agent-mode` button SHALL be disabled with the tooltip `Ask a workspace owner` / `Pide a un dueño del workspace`

#### Scenario: Policy denial surfaces inline

- **WHEN** the user attempts to start a session that the OPA policy denies (e.g., the workspace's ceiling forbids agent-mode for prod-tagged OpenSpecs)
- **THEN** the dock SHALL render the policy rationale inside the composer as a structured error block, and SHALL NOT navigate away

### Requirement: i18n parity for every visible string

Every label, placeholder, status badge, tooltip and toast inside the dock SHALL come from the i18n dictionary in ES and EN. No hard-coded user-facing strings SHALL appear in dock component source.

#### Scenario: Language switch updates dock copy live

- **WHEN** the user toggles the language pill from ES to EN while the dock is open
- **THEN** every visible dock string SHALL re-render in English without closing or losing the active session

### Requirement: Dock telemetry

The dock SHALL emit portal telemetry events for `dock_opened`, `dock_closed`, `dock_session_started`, `dock_follow_up_sent`, `dock_navigated_to_artifact` with the `correlation_id` of the active session when applicable.

#### Scenario: Opening the dock is auditable

- **WHEN** the user opens the dock
- **THEN** `portal.alfred.dock_opened.v1` SHALL be emitted with the principal, workspace, route, and the active `session_id` when present
