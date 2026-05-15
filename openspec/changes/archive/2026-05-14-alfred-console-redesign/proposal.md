## Why

The Alfred Console today is a single, developer-oriented surface: a slash-command palette + a long transcript of free-form Alfred responses. Non-technical users see an unfamiliar shell with cryptic identifiers (`/openspec`, `openspec_id`, `app_id`) and have no obvious entry point for the three jobs they actually want to do — *start a new app*, *improve an existing app*, *operate (deploy/observe) an existing app*. At the same time, the slash-command surface remains genuinely useful for developers who already know what they want to run. With **App as a first-class entity** now landed (`app-first-class-entity`), we have the anchor we need to (a) split the console into a Friendly view for non-tech users and an Advanced view for developers, (b) detect when an intent matches an existing App or spec via RAG so that we don't duplicate work, and (c) rename the legacy `/openspec` command to a brand-aligned, platform-agnostic `/forge`.

## What Changes

- Introduce two console **views**: **Friendly** (default for users with role `workspace.member` or below, and the platform default for first-time logins) and **Advanced** (default for users with role `workspace.developer` and above, opt-in for everyone else). Both views consume the same Alfred backend; the difference is the affordances, copy and visibility of IDs.
- **Friendly view**: a clean landing surface with three large cards — **Nueva App** (start a new app), **Mejorar** (extend an existing app), **Operar** (deploy / observe an existing app). Selecting a card opens a natural-language conversation with Alfred scoped to the relevant journey. The Friendly view SHALL never display raw IDs (`openspec_id`, `app_id`, slugs), SHALL not require slash commands, SHALL not surface OpenFGA error codes verbatim, and SHALL only show App picker as a friendly App switcher when more than one App exists in the user's workspace.
- **Advanced view**: the current Alfred Console preserved — slash-command palette, transcript, raw IDs displayed inline, full access to `/forge` and developer keyboard shortcuts. Adds an explicit App picker in the top of the console (selecting an App scopes every subsequent command).
- **View toggle**: an explicit "Modo desarrollador / Developer mode" toggle in user settings + a session-level toggle in the top of the console. The choice SHALL be persisted per user. Role-based default applies on first sign-in.
- **RAG-based spec deduplication on intent capture**: when the user describes an intent in either view, Alfred SHALL run a RAG retrieval against the App's spec corpus (and, if no App is selected yet, against the Workspace corpus). If the retrieval returns a top hit with `score >= 0.80`, Alfred SHALL present a dialog: *"Encontré un spec muy parecido: <título>. ¿Quieres extenderlo o crear uno nuevo?"*. The user picks **Extender** (the existing spec becomes the working spec, Alfred enters the agent-mode for that spec) or **Crear nuevo** (proceeds with a brand-new draft).
- **Direct-to-architect for committed specs**: when the matched spec is already `committed` (i.e., a full OpenSpec exists with `lifecycle_state` advanced beyond `proposed`), the dialog SHALL surface a primary **"Implementar"** action that bypasses the wizard and jumps the user directly into the `architect` step of the SDLC workflow for that spec.
- **Command rename**: rename `/openspec` to `/forge` across the platform. `/openspec` SHALL remain as a deprecated alias for two minor versions (announced in the release notes), emitting a deprecation toast on use and being removed in the third minor version after this change. Every doc and CLI reference SHALL be updated; the Friendly view never exposes either command.
- **Friendly conversation primitives**: Alfred replies in the Friendly view SHALL use the persona content rules already defined in `design/alfred-identity/PERSONA.md` (criticality glyph, citation footer, no emoji, ES/EN parity), and SHALL replace every raw ID with a human label (`app: hr-portal` → `la App "Time Off Tracker"`).

## Capabilities

### New Capabilities

- `alfred-console-friendly`: the Friendly view of the Alfred Console — landing cards, role-based default, natural-language conversation without IDs, App switcher, view toggle.
- `spec-deduplication`: RAG-based detection of similar/existing specs at intent-capture time, threshold-driven branching ("extend or create new"), and direct-to-architect routing for committed specs.

### Modified Capabilities

- `alfred-control-plane`: Alfred SHALL run the dedup retrieval as a mandatory step at intent capture; the dialogue API SHALL return a `spec_match` block when a match is found.
- `intent-capture-wizard`: when entered via the Friendly view, the wizard SHALL hide raw IDs and SHALL surface the dedup dialog before its first question.
- `portal-command-palette`: the palette SHALL register `/forge` as the primary command and `/openspec` as a deprecated alias; the palette SHALL surface a deprecation toast on `/openspec` use.
- `portal-shell`: the shell SHALL host the Friendly/Advanced view toggle in the user account menu and SHALL apply the role-based default on first sign-in.
- `alfred-agent-mode`: the agent-mode session SHALL accept a `start_step` hint (e.g., `start_step=architect`) so the "Implementar" action can skip discovery and jump to the architect step.

## Impact

- **Portal**: new Friendly view route (`/alfred` becomes the canonical entry point with two states `?view=friendly|advanced`); new landing cards component; updated dock and console copy; new App switcher in the Friendly view.
- **Alfred backend**: new `POST /v1/intent/match` endpoint that runs the RAG retrieval and returns matches with scores; modified `POST /v1/intent/start` to accept `bypass_match` for the "Crear nuevo" branch and `resume_spec_id` for the "Extender" branch; modified `POST /v1/agent-mode/sessions` to accept `start_step`.
- **Telemetry**: new events `alfred.console.view_toggled.v1`, `alfred.intent.match_found.v1`, `alfred.intent.match_dismissed.v1`, `alfred.command.deprecated_alias.v1`.
- **CLI**: `forge` is now the primary command; `openspec` continues to work and prints a deprecation warning.
- **Docs**: all CLAUDE.md / README references to `/openspec` updated; deprecation window documented; PERSONA.md referenced from the Friendly view copy guidelines.
- **Migration**: per-user view preference defaults: `workspace.member` → Friendly; `workspace.developer` and above → Advanced. First-time users always start in Friendly with a "switch to developer" hint at the top.
