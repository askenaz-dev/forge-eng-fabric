# alfred-identity Specification

## Purpose

Alfred's named brand surface inside the Forge design system — SVG mark,
persona voice rules, color and motion tokens, content guidelines for
agent-authored messages, and placement do/don'ts. Created by archiving
change `alfred-agent-mode-orchestrator`.

## Requirements

### Requirement: Named visual identity in the Forge design system

The Forge design system SHALL include **Alfred** as a named brand surface alongside the Forge wordmark. The identity SHALL include a primary SVG mark, a monochrome mark, an animated "working" mark, a typographic lockup pairing the Forge family (Instrument Serif italic + mono eyebrow) with Alfred's persona name, and persona color tokens. All assets SHALL be authored as source SVGs in `design/alfred-identity/marks/` and exported as production SVGs to `portal/public/`.

#### Scenario: Mark sources are available in the design folder

- **WHEN** a designer inspects `design/alfred-identity/marks/`
- **THEN** the folder SHALL contain `alfred-mark.svg` (full color), `alfred-mark-mono.svg` (single ink), `alfred-mark-working.svg` (animation source) and an `alfred-mark.notes.md` explaining geometry, ember/bowtie motif and the relationship to the Forge `F.` mark

#### Scenario: Production marks are available to the portal

- **WHEN** the portal imports the mark from `/alfred-mark.svg` or its monochrome / working variants
- **THEN** the file SHALL exist under `portal/public/`, ship optimized SVG (no editor metadata, viewBox normalized to 24×24), and render at 16, 24, 32 and 48 px without visible distortion

### Requirement: Persona voice and content rules for Alfred-authored messages

The design system SHALL document Alfred's persona voice — calm, precise, slightly butler-formal, citation-first — and the content rules that any Alfred-authored UI surface SHALL follow: prefix critical actions with a criticality glyph, cite the source OpenSpec / runbook / policy when reasoning, never use exclamation marks, never use emojis, default to second-person in ES (tú) and EN (you).

#### Scenario: Persona note is published in the design folder

- **WHEN** a contributor opens `design/alfred-identity/PERSONA.md`
- **THEN** the document SHALL list voice rules, copy do/don'ts with at least 3 worked examples per do/don't, ES and EN parallel phrasing, and the canonical criticality glyphs (`◇ low`, `◆ medium`, `■ high`, `▲ critical`)

#### Scenario: Dock copy passes a persona lint

- **WHEN** CI runs the persona lint over `portal/src/i18n/dictionary.ts` keys prefixed `alfred.`
- **THEN** the lint SHALL fail on any exclamation mark, emoji, first-person plural ("we"), or missing ES/EN pair

### Requirement: Color and motion tokens

Alfred SHALL ship as a named token group `--alfred-*` in the design tokens contract: `--alfred-ink`, `--alfred-paper`, `--alfred-ember` (call-to-attention accent), `--alfred-thread` (links/cites), `--alfred-working` (in-progress animation accent). Motion SHALL be tokenized as `--alfred-dock-ease` (cubic-bezier(0.2, 0.8, 0.2, 1)), `--alfred-dock-in-ms` (220ms), `--alfred-dock-out-ms` (160ms), `--alfred-working-cycle-ms` (1600ms).

#### Scenario: Tokens resolve in both themes

- **WHEN** the portal renders the dock in light and dark themes
- **THEN** every `--alfred-*` token SHALL have a defined value in `:root` and `[data-theme="dark"]`, and contrast ratios for `--alfred-ink` on `--alfred-paper` SHALL meet WCAG AA for normal text in both themes

#### Scenario: Motion is reduced-motion aware

- **WHEN** the user has `prefers-reduced-motion: reduce`
- **THEN** the working animation SHALL fall back to a still mark and the dock open/close SHALL use opacity-only transitions of `≤ 80ms`

### Requirement: Do / don't placement board

The design system SHALL publish a do/don't board for the Alfred dock and mark covering: minimum clearance from viewport edges, prohibition on duplicating the launcher on the same route, prohibition on placing the mark over imagery without a tinted scrim, and the rule that Alfred-authored chat bubbles always render the criticality glyph at line start.

#### Scenario: Designer references the do/don't board

- **WHEN** a designer opens `design/alfred-identity/DO_DONT.md`
- **THEN** the document SHALL contain at least 6 do/don't pairs each illustrated by an inline SVG, and SHALL be linked from the Brand Notebook standalone HTML under a new Alfred section

### Requirement: Brand Notebook standalone reflects Alfred

The standalone `design/Forge Brand Notebook _standalone_.html` SHALL gain a new section titled `Alfred` between the existing brand and component sections, embedding the three marks, the persona note summary, the color/motion tokens, and a thumbnail of the do/don't board. The notebook SHALL remain a single-file standalone HTML (no external network dependencies).

#### Scenario: Notebook is self-contained and includes Alfred

- **WHEN** a reviewer opens the standalone HTML offline
- **THEN** the Alfred section SHALL render with embedded SVG marks and inline CSS for the persona tokens, and SHALL match the same visual rhythm as the surrounding Forge sections
