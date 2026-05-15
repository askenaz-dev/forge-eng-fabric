## ADDED Requirements

### Requirement: Friendly view as the default surface for non-tech users

The Alfred Console SHALL expose a **Friendly view** at `/alfred?view=friendly` (and at `/alfred` when the user's preference resolves to Friendly). The Friendly view SHALL be the default surface for any user whose effective workspace role is `workspace.member`, `workspace.viewer` or unset on first sign-in. The view SHALL NOT display raw identifiers (`openspec_id`, `app_id`, slugs), SHALL NOT require slash-command knowledge, and SHALL NOT surface backend error codes verbatim — every recognised backend error SHALL be translated to a human-readable copy block.

#### Scenario: First-time non-developer lands on Friendly view

- **GIVEN** a newly created user `user-1` with `workspace.member` on `ws-1` and no `user.console_view_preference`
- **WHEN** `user-1` signs in for the first time and navigates to `/alfred`
- **THEN** the Portal MUST render the Friendly view by default and persist `user.console_view_preference=friendly`

#### Scenario: Backend error is translated to friendly copy

- **WHEN** the Friendly view receives `403 missing_app_editor` from any Alfred API call
- **THEN** the UI MUST render "No tienes permisos para editar esta App. Pide acceso a <owner_name>." / "You don't have permission to edit this app. Ask <owner_name>."
- **AND** a "Detalles técnicos" / "Show technical details" disclosure MUST allow showing the raw error payload

### Requirement: Friendly view shows three landing cards

The Friendly view's landing surface SHALL render exactly three large cards:

1. **Nueva App** (`new_app`) — "Crea una app nueva desde cero" / "Start a brand new app"
2. **Mejorar** (`improve_app`) — "Añade o cambia algo en una app existente" / "Add or change something in an existing app"
3. **Operar** (`operate_app`) — "Despliega, supervisa o resuelve un problema" / "Deploy, monitor or troubleshoot"

Selecting a card SHALL open a scoped conversation panel with Alfred. The card title SHALL be Instrument Serif italic, the body in Geist; the cards SHALL meet the design-system contrast contract.

#### Scenario: Cards are rendered with the canonical copy

- **WHEN** a user with Friendly view loads `/alfred`
- **THEN** the landing MUST render three cards with the titles and bodies above, in ES if `lang=es-CO`, EN if `lang=en-US`
- **AND** each card MUST have a single primary "Empezar" / "Start" CTA

#### Scenario: Selecting Nueva App opens a conversation scoped to new-app

- **WHEN** the user selects "Nueva App"
- **THEN** the panel MUST open an Alfred conversation with the `new_app` prompt template
- **AND** the conversation MUST display Alfred's first message asking for the App's purpose in plain language
- **AND** the panel MUST NOT show any slash command affordance

### Requirement: Friendly view replaces raw IDs with human labels

In the Friendly view, every reference to an entity SHALL be rendered using the entity's human-readable label, computed by the Portal's label resolver. Internally, the React state holds the IDs (so API calls remain typed and correct), but the rendered text MUST be the label. If a label cannot be resolved, the Portal MUST render an italic placeholder ("una App reciente" / "a recent App") and never the raw ID.

#### Scenario: App referenced by label not by ID

- **GIVEN** the user is in the Friendly view with `app_id=app-1` whose `name="Time Off Tracker"`
- **WHEN** Alfred mentions the App in a transcript message
- **THEN** the rendered text MUST be 'la App "Time Off Tracker"' / 'the "Time Off Tracker" app'
- **AND** the raw `app-1` ID MUST NOT appear in the DOM text

#### Scenario: Label resolver fails gracefully

- **WHEN** the Portal cannot resolve the label for an entity (network error, etc.)
- **THEN** the UI MUST render the italic placeholder "una App reciente" instead of the raw ID
- **AND** an error MUST be logged to the Portal telemetry stream

### Requirement: Friendly App switcher

When the user's current workspace has two or more Apps visible to them (excluding the system-managed `_unassigned`), the Friendly view SHALL render a compact **App switcher** at the top of the conversation panel. The switcher SHALL show the current App's name and an icon; clicking it opens a popover with the other Apps. When there is only one App, the switcher SHALL be replaced by a static "App: <name>" label.

#### Scenario: Multiple apps shows a switcher

- **GIVEN** a workspace with two visible Apps `app-a` and `app-b`
- **WHEN** the user opens the Friendly view
- **THEN** the conversation panel header MUST render a switcher button with the active App's name and a chevron
- **AND** clicking the switcher MUST open a popover listing both Apps with their names and last activity

#### Scenario: Single app shows a static label

- **GIVEN** a workspace with exactly one visible App
- **WHEN** the user opens the Friendly view
- **THEN** the header MUST render "App: <name>" as static text (no popover affordance)

### Requirement: View toggle in user settings

The Portal SHALL expose a **Modo desarrollador / Developer mode** toggle in the user account menu. Toggling it SHALL switch the console view between Friendly and Advanced and persist the choice via `PUT /api/user/preferences {console_view_preference}`. A session-level switch SHALL also be available at the top of the console; this session switch SHALL persist for the current browser session only unless the user also clicks "Save as my default".

#### Scenario: Toggle persists across sessions

- **WHEN** a Friendly-view user toggles "Modo desarrollador" on
- **THEN** `user.console_view_preference` MUST flip to `advanced`
- **AND** the next sign-in MUST land directly on the Advanced view

#### Scenario: Session switch defaults to ephemeral

- **WHEN** a Friendly-view user clicks the session-level switch at the top of the console
- **THEN** the Advanced view MUST render for the rest of the browser session
- **AND** `user.console_view_preference` MUST remain `friendly` unless the user explicitly clicks "Save as my default"

### Requirement: Friendly view emits view-toggle audit events

The Portal SHALL emit `alfred.console.view_toggled.v1` with `{principal, from, to, persistent}` every time the view changes (either persistent toggle or session switch). The event SHALL flow through the existing portal audit pipeline.

#### Scenario: Toggle emits audit event

- **WHEN** a user toggles from Friendly to Advanced persistently
- **THEN** the audit event MUST carry `{from:"friendly", to:"advanced", persistent:true}`

### Requirement: Tenant-level default override

A tenant administrator MAY set `tenant.console_default_view` to `friendly` or `advanced`. When set, the tenant default SHALL override the role-based default for first-time sign-ins. The tenant default SHALL NOT override a user's already-persisted preference.

#### Scenario: Tenant default overrides role-based default

- **GIVEN** `tenant.console_default_view=friendly` and a user `user-2` with `workspace.developer` and no `user.console_view_preference`
- **WHEN** `user-2` signs in for the first time
- **THEN** the Portal MUST render Friendly view (overriding the developer-default Advanced)
- **AND** persist `user.console_view_preference=friendly`
