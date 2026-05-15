## ADDED Requirements

### Requirement: `/forge` is the primary console command, `/openspec` is a deprecated alias

The portal command palette and the `forge` CLI SHALL accept `/forge` as the primary, canonical command for every operation that previously lived under `/openspec`. The legacy `/openspec` command SHALL continue to work as an alias for **two minor versions** after this change ships. While the alias is active, every invocation SHALL:

- emit a deprecation toast in the Portal palette: "El comando /openspec se renombró a /forge. Úsalo a partir del próximo release." / "/openspec has been renamed to /forge. Please use /forge going forward."
- print `WARNING: /openspec is deprecated; use /forge` in the CLI output
- emit an `alfred.command.deprecated_alias.v1` audit event with `{principal, original_input, mapped_to, view, version}`

In the **third minor version** after this change, the alias SHALL be removed; calling `/openspec` SHALL return `404 deprecated_command_removed` with a hint pointing at `/forge`.

#### Scenario: Palette accepts /forge

- **WHEN** the user opens the command palette and types `/forge new`
- **THEN** the palette MUST surface the `new` subcommand of `/forge` and execute it on `Enter`
- **AND** the audit event MUST be `portal.command.invoked.v1` with `target_id="forge.new"`

#### Scenario: /openspec emits deprecation warning

- **WHEN** the user types `/openspec list`
- **THEN** the palette MUST execute the command (mapped to `/forge list`) AND render a yellow deprecation toast with the canonical copy
- **AND** an `alfred.command.deprecated_alias.v1` event MUST be emitted with `{original_input: "/openspec list", mapped_to: "/forge list"}`

#### Scenario: /openspec removed in third minor

- **GIVEN** the platform is running the third minor version after this change shipped
- **WHEN** any caller invokes `/openspec`
- **THEN** the platform MUST respond `404 deprecated_command_removed` with the hint message
- **AND** no command execution MUST proceed

### Requirement: Friendly view does not register either command in the palette

When the user's effective console view is Friendly, the command palette SHALL hide both `/forge` and `/openspec` from the registered command list. Typing `/` in the Friendly view SHALL NOT open the palette — the keystroke SHALL be delivered to the current input as text.

#### Scenario: Friendly view ignores the / shortcut

- **GIVEN** a Friendly-view user on `/alfred`
- **WHEN** the user types `/` with no input focused
- **THEN** the command palette MUST NOT open
- **AND** the keystroke MUST not be globally captured

#### Scenario: Advanced view enables both commands

- **GIVEN** an Advanced-view user
- **WHEN** the user opens the command palette and types `/`
- **THEN** the palette MUST show `/forge` as the primary entry and `/openspec` as a "(deprecated)" entry below it (during the deprecation window)
