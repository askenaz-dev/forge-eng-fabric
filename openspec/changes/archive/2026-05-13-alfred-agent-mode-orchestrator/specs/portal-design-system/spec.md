## ADDED Requirements

### Requirement: Alfred token group registered in the design tokens

The Portal design tokens contract SHALL include an `--alfred-*` group documented in the design system reference. The group SHALL ship light and dark values for every token and SHALL be referenced from `portal/src/app/globals.css`.

#### Scenario: Tokens are documented and resolvable

- **WHEN** a contributor reads the design system reference for tokens
- **THEN** the document SHALL list `--alfred-ink`, `--alfred-paper`, `--alfred-ember`, `--alfred-thread`, `--alfred-working`, `--alfred-dock-ease`, `--alfred-dock-in-ms`, `--alfred-dock-out-ms`, `--alfred-working-cycle-ms` with light/dark values and usage guidance
- **AND** these tokens SHALL be resolvable via `getComputedStyle(document.documentElement)` in both themes

### Requirement: Mark registry includes Alfred variants

The shared mark registry consumed by the shell and the brand notebook SHALL include the three Alfred marks (`alfred-mark`, `alfred-mark-mono`, `alfred-mark-working`) keyed by stable identifiers so downstream surfaces (notifications, audit excerpts, run cards) can render the same mark without reimporting SVG files.

#### Scenario: Mark is reachable from shell components

- **WHEN** a shell component imports the mark by id `alfred-mark`
- **THEN** the registry SHALL return a React component that renders the inline SVG with `viewBox="0 0 24 24"`, accepts `size`, `color` and `aria-label` props, and matches the Forge mark API

### Requirement: Persona-aware copy rules apply to Alfred-authored surfaces

Any Portal surface that renders messages authored by Alfred (dock transcript, agent-mode run cards, notification toasts originating from Alfred) SHALL apply the persona content rules from `design/alfred-identity/PERSONA.md`: criticality glyph prefix, citation footer, no emoji, no exclamation marks, ES/EN parity.

#### Scenario: Run card renders criticality glyph and citation

- **WHEN** the portal renders an Alfred-authored decision summary inside a run card
- **THEN** the summary SHALL start with the criticality glyph mapped from `low|medium|high|critical`, and SHALL include a citation footer linking to the source OpenSpec, runbook or policy used in the decision
