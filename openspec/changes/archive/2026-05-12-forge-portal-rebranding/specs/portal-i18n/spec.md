## ADDED Requirements

### Requirement: Bilingual content layer with Spanish default

The Portal SHALL ship with two supported locales — `es` (default) and `en` — backed by a static, type-safe dictionary at `portal/src/i18n/dictionary.ts`. The dictionary SHALL define every translatable string keyed by short identifiers (e.g. `nav_dashboard`, `h_invite`, `apr_approve`, `toast_theme`, `kpi_p95`, `runs_filter_running`) and SHALL parallel the keys present in the Brand Notebook source `design/fabric-unzipped/i18n.jsx`. Every key SHALL exist for both locales — missing keys SHALL fall back to `es` and SHALL be reported as a build-time error.

#### Scenario: Both locales define every key

- **WHEN** the i18n parity check (`pnpm --filter @forge/portal run i18n:check`) runs in CI
- **THEN** every key present in `es` is present in `en` and vice versa
- **AND** any divergence causes the script to exit non-zero with a list of the missing keys

#### Scenario: Default lang on first visit is Spanish

- **WHEN** a new user (no `forge_lang` in localStorage, no profile preference) opens the Portal
- **THEN** `<html lang="es">` is rendered server-side and the dashboard headline reads "Tu telar está corriendo en caliente."

### Requirement: useLang hook and t() formatter

The Portal SHALL expose a React hook `useLang()` returning `[lang, setLang, t]` where `t(key, vars?)` resolves a key with optional `{var}` substitution (e.g. `t("ev_run_started", { agent: "review-bot", repo: "acme/payments-svc" })`). The hook SHALL be backed by a `LangProvider` that persists `lang` to `localStorage.forge_lang`, sets `document.documentElement.lang`, and exposes a stable `t` reference that re-renders all consumers on language change.

#### Scenario: t() substitutes variables

- **WHEN** code calls `t("ev_apr_granted", { target: "orders-api", who: "ana@acme.io" })` under `lang === "en"`
- **THEN** the result is the string `Approved change on orders-api by ana@acme.io`

#### Scenario: setLang re-renders all consumers

- **WHEN** the user toggles the ES/EN pill
- **THEN** every component that called `useLang()` re-renders with the new locale within one frame
- **AND** server-rendered subtrees that depend on `lang` revalidate on next navigation via `router.refresh()`

### Requirement: Server-rendered initial language

Initial server render SHALL use the language from (in order): the `Accept-Language` header for the first ever visit, then the persisted profile preference fetched from `GET /v1/users/me/preferences`, then a `forge_lang` cookie set by the Portal. The chosen language SHALL be passed to the client as a hydration prop to avoid a language flash during hydration.

#### Scenario: Returning user's stored EN choice is honoured on the initial paint

- **WHEN** a user whose profile preference is `en` makes a fresh request to `/`
- **THEN** the initial HTML uses English copy on every shell label and dashboard string
- **AND** no flash of Spanish content occurs during hydration

### Requirement: Language preference is persisted server-side

When `setLang(next)` is called, the Portal SHALL `POST /api/i18n/preference { "lang": next }` which writes to `users.preferences.locale` via the user-preferences endpoint of the control-plane. Toasts SHALL confirm the switch (`toast_lang` / `toast_lang_en`) and a `portal.lang.changed` audit event SHALL be emitted with the principal, previous and next locales.

#### Scenario: Lang switch is audited

- **WHEN** a user switches from `es` to `en`
- **THEN** the audit log contains an entry `{ "type": "portal.lang.changed", "principal": "...", "from": "es", "to": "en", "correlation_id": "..." }`

### Requirement: Pluralisation and number/date formatting

The `t()` API SHALL accept `count` to select a plural form (`t("n_runs", { count: 1 })` → "1 run" / "1 run", `count: 3` → "3 runs" / "3 runs"). Number and date formatting SHALL use the Intl APIs with the active locale (`new Intl.NumberFormat(lang)`, `new Intl.DateTimeFormat(lang, { dateStyle: 'medium', timeStyle: 'short' })`). Durations expressed in seconds SHALL be formatted as `mm:ss` regardless of locale.

#### Scenario: Locale-aware date in the activity timeline

- **WHEN** the active locale is `es` and an event timestamp is `2026-05-12T14:30:00Z`
- **THEN** the displayed value is `12 may 14:30` (24h, lowercase month) using `Intl.DateTimeFormat('es')`
- **AND** under `en` the same timestamp renders as `May 12, 2:30 PM`

### Requirement: ES/EN parity for all platform terms

Domain-specific terms used in the dictionary SHALL agree with the platform glossary:

- `Tablero` ↔ `Dashboard`
- `Aprobaciones` ↔ `Approvals`
- `Specs (OpenSpec)` ↔ `Specs (OpenSpec)` (no translation; product term)
- `Políticas (OPA)` ↔ `Policies (OPA)`
- `Auditoría` ↔ `Audit`
- `Métricas y trazas` ↔ `Metrics & traces`
- `Mesh de servicios` ↔ `Service mesh`
- `Human-in-the-loop · OPA` ↔ `Human-in-the-loop · OPA`
- `Cola de aprobación` ↔ `Approval queue`
- `Runs en curso` ↔ `Runs in flight`
- `Éxito 24 h` ↔ `Success 24 h`
- `p95 latencia` ↔ `p95 latency`
- `Horas ahorradas / semana` ↔ `Hours saved / week`
- `Lanzar workflow` ↔ `Launch workflow`
- `Invitar equipo` ↔ `Invite team`

Any new key added to the dictionary SHALL appear in the glossary section of `docs/portal/i18n.md`.

#### Scenario: Glossary parity audit

- **WHEN** the docs CI runs `pnpm --filter @forge/portal run i18n:glossary-audit`
- **THEN** every term in the table above resolves to the canonical translation in both directions, with zero discrepancies
