## ADDED Requirements

### Requirement: Persistent Alfred dock affordance

The portal shell SHALL mount the Alfred dock as a persistent shell affordance on every authenticated route, anchored to the bottom-right of the viewport. The dock SHALL render inside the same React tree as the shell (alongside the toast rail and command palette) so it survives client-side route transitions without unmount.

#### Scenario: Dock survives route transitions

- **WHEN** the user navigates from `/` to `/agents` while the dock is open with an active session
- **THEN** the dock SHALL remain open, the SSE stream SHALL remain connected, and the transcript SHALL NOT be reset
- **AND** the new route's content SHALL render normally inside `<main class="main">` without layout shift attributable to the dock

#### Scenario: Dock z-layer is above toasts and below modals

- **WHEN** a toast and a modal dialog are both visible at the same time as the open dock
- **THEN** the dock SHALL render above the toast rail and below the modal overlay
- **AND** focusing the modal SHALL not leak keyboard focus into the dock

### Requirement: Shell exposes the dock-gate permission check

The shell SHALL fetch the principal's `alfred:invoke` and `alfred:agent-mode.run` permissions as part of `GET /api/permissions/me` and SHALL pass them to the dock so the launcher and the start button render in the correct disabled/enabled state without an extra round trip.

#### Scenario: Permissions endpoint includes Alfred scopes

- **WHEN** the shell calls `GET /api/permissions/me`
- **THEN** the response SHALL include the boolean fields `alfred_invoke` and `alfred_agent_mode_run` for the active workspace

#### Scenario: Launcher hidden for unprivileged principals

- **WHEN** the principal lacks `alfred:invoke` for the active workspace
- **THEN** the shell SHALL NOT render the launcher button at all, and no SSE connection SHALL be opened

### Requirement: Keyboard summon coexists with command palette

The shell SHALL register the Alfred hotkey at the same precedence layer as the command palette hotkey. When one surface is open, the hotkey for the other SHALL close the open surface and open the requested one.

#### Scenario: Cmd-K closes the dock before opening the palette

- **WHEN** the dock is open and the user presses `Ctrl K` / `⌘K`
- **THEN** the dock SHALL close and the command palette SHALL open
- **AND** pressing `Alt A` / `⌥A` from inside the palette SHALL reverse the swap

### Requirement: Shell-level workspace switch propagates to the dock

When the user switches workspace from the sidebar tenant picker, the dock SHALL re-target its session stream to the newly active workspace and prompt the user to confirm any in-progress follow-up before discarding it.

#### Scenario: Workspace switch discards stale session view

- **WHEN** the dock is open showing a session for workspace A and the user switches the active workspace to B
- **THEN** the dock SHALL close the SSE connection for A, reload state for B, and prepend an info banner `Switched to <B>` / `Cambiado a <B>` retained for 5 seconds
- **AND** any unsent follow-up text SHALL be preserved in the composer along with a `Discard / Mantener` confirmation
