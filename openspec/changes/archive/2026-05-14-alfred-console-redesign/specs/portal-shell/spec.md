## ADDED Requirements

### Requirement: Console view toggle in the account menu

The Portal account menu (in the top bar's user avatar popover) SHALL include a **Modo desarrollador / Developer mode** toggle. The toggle SHALL reflect the user's `console_view_preference` (off when `friendly`, on when `advanced`) and SHALL persist the choice via `PUT /api/user/preferences { console_view_preference }`. The toggle SHALL emit `alfred.console.view_toggled.v1`.

#### Scenario: Toggle reflects current preference

- **GIVEN** a user with `console_view_preference=advanced`
- **WHEN** the user opens the account menu
- **THEN** the toggle MUST render as "on"

#### Scenario: Flipping the toggle persists immediately

- **WHEN** the user flips the toggle from off to on
- **THEN** the Portal MUST issue `PUT /api/user/preferences { console_view_preference: "advanced" }`
- **AND** the active route MUST switch to the Advanced view without a full reload
- **AND** an `alfred.console.view_toggled.v1` event MUST be emitted

### Requirement: Role-based default for first-time sign-in

When a user signs in for the first time and `user.console_view_preference` is unset, the Portal SHALL resolve the default view in this order:

1. If `tenant.console_default_view` is set, use it.
2. Otherwise, if the user has `workspace.developer` or higher on any of their workspaces, use Advanced.
3. Otherwise, use Friendly.

The resolved value SHALL be persisted to `user.console_view_preference` on first sign-in.

#### Scenario: First-time member defaults to Friendly

- **GIVEN** a freshly created user `user-1` with only `workspace.member` on `ws-1`, no tenant default
- **WHEN** `user-1` signs in for the first time
- **THEN** `user.console_view_preference` MUST be set to `friendly`
- **AND** the Portal MUST land on the Friendly view

#### Scenario: First-time developer defaults to Advanced

- **GIVEN** a freshly created user `user-2` with `workspace.developer` on `ws-1`, no tenant default
- **WHEN** `user-2` signs in for the first time
- **THEN** `user.console_view_preference` MUST be set to `advanced`
- **AND** the Portal MUST land on the Advanced view

#### Scenario: Tenant default takes precedence

- **GIVEN** `tenant.console_default_view=friendly` and a developer `user-2` signing in for the first time
- **THEN** `user.console_view_preference` MUST be set to `friendly`
