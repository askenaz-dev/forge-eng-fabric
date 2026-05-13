## ADDED Requirements

### Requirement: Time-of-day greeting and serif headline

The Tablero landing page SHALL render a greeting that varies by local time (`h_hello` 05:00–11:59, `h_hello_pm` 12:00–17:59, `h_hello_n` 18:00–04:59) followed by the user's first name in mono-eyebrow case, and a serif italic-accent display headline (Instrument Serif). The headline copy SHALL be templated from the i18n keys `h_overview_pre`, `h_overview_em`, `h_overview_post`, dynamically interpolating the live count of active agents and pending approvals.

#### Scenario: Afternoon visit greets with "Buenas tardes"

- **WHEN** the user opens the dashboard at 14:30 local time with locale `es`
- **THEN** the eyebrow above the headline reads `BUENAS TARDES, ANA`
- **AND** the headline reads `Tu telar está corriendo en caliente.` with `telar` styled italic and ember-coloured

#### Scenario: Headline reports live counts

- **WHEN** the dashboard mounts and `GET /v1/registry/assets?kind=agent&status=active` returns 14 and `GET /v1/approvals?status=pending&approver=<me>` returns 3
- **THEN** the subheading reads `14 agentes activos, 3 aprobaciones esperándote.` / `14 agents active, 3 approvals waiting.`

### Requirement: Primary call-to-action buttons

Below the headline the dashboard SHALL render two CTA buttons: secondary "Invitar equipo" / "Invite team" (opens a modal that issues an invite via `POST /v1/tenants/{id}/invites`) and primary ember "Lanzar workflow" / "Launch workflow" (opens the workflow picker linked to `/workflows`). Buttons SHALL use the design-system `Button` primitive.

#### Scenario: Lanzar workflow opens the picker

- **WHEN** the user clicks "Lanzar workflow"
- **THEN** a modal lists workflows from `GET /v1/workflows?published=true`, the user picks one, and on confirm a `POST /v1/sdlc/runs` creates a run; the dashboard appends the new run to the "Runs recientes" table without a full reload

### Requirement: KPI grid

The dashboard SHALL render a four-column KPI grid populated from `GET /v1/observability/kpis?window=24h` proxied through `/api/observability/kpis`. The four KPIs SHALL be:

- **Runs en curso / Runs in flight**: `runs_in_flight` count, primary-tinted sparkline of the past 12 hourly buckets, delta vs prior week, icon `workflows`.
- **Éxito 24 h / Success 24 h**: `success_rate_24h` percentage with one decimal, thread-tinted sparkline, delta in `pts`, icon `check`.
- **p95 latencia / p95 latency**: `p95_ms` integer with `ms` unit, info-tinted sparkline, delta in `ms`, icon `clock`, label `avg today`.
- **Horas ahorradas / semana / Hours saved / week**: `hours_saved` integer with `h` unit, copper-tinted sparkline, delta in `h`, icon `bolt`.

Each KPI SHALL use the design-system `Kpi` primitive with mono eyebrow, Instrument Serif display number and italic small unit. Sparklines SHALL be SVG paths derived from the `samples` array (12 points).

#### Scenario: KPI binds to live endpoint

- **WHEN** the dashboard mounts
- **THEN** `/api/observability/kpis?window=24h` is called server-side once per request, the response is rendered into the four KPI cards, and the response time appears in the page-source view's `x-correlation-id` for traceability

#### Scenario: KPI handles partial unavailability

- **WHEN** `p95_ms` is null because the metrics backend reported `no data`
- **THEN** the p95 KPI renders `—` for the number, mutes the sparkline opacity to 30%, and shows an "sin datos" / "no data" eyebrow chip

### Requirement: Runs recientes table

The dashboard SHALL render the "Runs recientes" table fed by `GET /v1/sdlc/runs?limit=50&order=desc` with the schema described below per row:

- status pulse dot (`thread` for running/success pulse, `spark` for warn, `rust` for failed, `info` for pending, `fg-3` for queued)
- agent block: small mono tag from agent initials + name (sans medium) + small mono subtitle showing the agent slug and run id
- repo (mono)
- duration (mono, tabular numerals)
- status badge (matching tone)
- policy slug (mono small)
- chevron

Rows SHALL be clickable to open the run sheet. The table SHALL be preceded by filter chips: `Todos`, `Corriendo`, `Esperando aprobación`, `Exitosos`, `Fallidos`, each with a live count from the same response.

#### Scenario: Filter chips reflect counts

- **WHEN** the runs response contains 50 entries with statuses `{running: 12, pending: 3, success: 30, failed: 4, queued: 1}`
- **THEN** the chips display `Todos 50`, `Corriendo 12`, `Esperando aprobación 3`, `Exitosos 30`, `Fallidos 4`
- **AND** clicking `Fallidos` filters the table to the 4 failed runs without a network round-trip

#### Scenario: Row click opens run sheet with steps

- **WHEN** the user clicks a row for `wf_8a13c1`
- **THEN** the right-side sheet opens with header `Run · Deploy v1.42.0 → prod` and body populated from `GET /v1/sdlc/runs/wf_8a13c1` containing the step list, durations and tones, and the URL bar updates to `/?run=wf_8a13c1`

### Requirement: Cola de aprobación stack

The dashboard SHALL render the approval queue card sourced from `GET /v1/approvals?status=pending&approver=<me>&limit=10` showing for each item: the requesting agent's two-letter tag, title (localised), meta (run id, branch, scope), severity badge (`high` if applicable), diff summary (`+N / −M` and one-line summary), inline action buttons `Aprobar` / `Revisar` / `Rechazar`, and an expiry timer counting down to `expires_at`. Approving / rejecting SHALL call `POST /v1/approvals/{id}/decisions` and remove the item from the list on success.

#### Scenario: Approving removes the item and emits an audit event

- **WHEN** the user clicks `Aprobar` on `apr_2401`
- **THEN** a `POST /v1/approvals/apr_2401/decisions {"decision":"approve","actor":"<me>"}` is sent, the row animates out, the sidebar pill count for `Aprobaciones` decrements by one, and `approvals.granted.v1` appears in the audit stream

#### Scenario: Empty state shows "Sin aprobaciones pendientes"

- **WHEN** the approver has no pending requests
- **THEN** the card body renders the empty state from the i18n key `apr_no_items`

### Requirement: Actividad de la plataforma timeline

The dashboard SHALL render a signed-events timeline sourced from `GET /v1/audit/events?limit=20&scope=workspace` with iconography keyed to event type:

- `agent.run.started.v1` → `play` icon, ember tone
- `approvals.granted.v1` → `check` icon, thread tone
- `policy.denied.v1` → `shield` icon, rust tone
- `assets.skill.published.v1` → `skills` icon, ember tone
- `self_healing.action.taken.v1` → `bolt` icon, ember tone
- `openspec.merged.v1` → `specs` icon, thread tone

Each row SHALL render the event text from the i18n key `ev_*` with variables bolded inline (e.g. `Agent **review-bot** started a run on **acme/payments-svc**`), and a relative "when" stamp (e.g. `hace 2 min` / `2 min ago`).

#### Scenario: Variable substitution renders code-styled spans

- **WHEN** the audit returns `agent.run.started.v1` with vars `{agent:"review-bot", repo:"acme/payments-svc"}`
- **THEN** the row text reads `Agent <code>review-bot</code> started a run on <code>acme/payments-svc</code>` with the code-styled inserts in mono

#### Scenario: Relative time updates in place

- **WHEN** a row is rendered with timestamp 90 seconds ago
- **THEN** it reads `hace 1 min` and self-updates to `hace 2 min` after one minute without re-fetching

### Requirement: Mesh de servicios visualization

The dashboard SHALL render the service-mesh card with a hub-spoke SVG diagram. The orchestrator (`sdlc-orchestrator`) SHALL be the centre node with an ember glow; spokes SHALL connect to `policy-svc`, `openfga`, `registry`, `audit`, `context-eng`, `spec-engine` and `pgvector`. Each node SHALL render a pulsing inner circle whose colour reflects health: `thread` for healthy, `spark` for degraded, `rust` for down. Service health SHALL come from `GET /v1/observability/services/health`. Below the SVG, the four busiest services (by rps) SHALL be tiled with mono `nm`, kind label and live rps / p99 chips.

#### Scenario: Degraded service shows amber pulse

- **WHEN** `GET /v1/observability/services/health` reports `openfga` as `degraded`
- **THEN** the openfga spoke node colour becomes `var(--spark)`, the legend chip "1 degradado" / "1 degraded" appears, and clicking the node opens the service detail sheet

### Requirement: Live refresh and SSE updates

The dashboard SHALL refresh KPIs, runs, approvals, activity and service health on the following triggers:

- Initial server render (no client refresh needed in the first 2 seconds).
- Re-fetch on `router.refresh()` after workspace switch.
- SSE updates from `/api/notifications/stream` keyed on event type:
  - `agent.run.*.v1` → invalidate runs query and KPI `runs_in_flight`
  - `approvals.requested.v1` / `approvals.granted.v1` / `approvals.denied.v1` → invalidate approvals and sidebar count
  - `audit.event.*` → prepend to activity timeline
  - `observability.kpi.updated` → invalidate KPI

The eyebrow under each card SHALL show "en vivo · refresca cada Ns" reflecting either the SSE pulse or a fallback polling interval (default 30s). No fixed-interval poller SHALL be active when SSE is connected.

#### Scenario: New run via SSE appears at the top

- **WHEN** SSE emits `agent.run.started.v1 { run_id: "wf_8a13d0", agent: "doc-weaver", ... }`
- **THEN** within one frame the new row is prepended to the runs table with a brief ember highlight that fades after 1.2s, and the `Runs en curso` KPI ticks up by one

### Requirement: Real data only

The dashboard SHALL not contain any inline mock array, hard-coded fixture data, or string literal that begins with `mock_`. All values, including the agent initial tags used in row prefixes, SHALL be derived from real server payloads. Skeleton shimmer placeholders SHALL be used during initial load — they SHALL NOT contain demo content (no fake names, no fake repos, no fake ids).

#### Scenario: Audit confirms no fixture data

- **WHEN** `rg -n "mock_|fixture|fake|lorem|Ana Restrepo|wf_8a13" portal/src/app/` runs after merge
- **THEN** zero matches are found

#### Scenario: Skeleton loading is anonymous

- **WHEN** the dashboard mounts and the runs endpoint is still pending
- **THEN** the table renders 5 grey shimmer rows with no visible text content

### Requirement: Performance and instrumentation

The dashboard SHALL render the initial server HTML in ≤ 300ms p95 with the live stack reachable, and SHALL achieve Largest Contentful Paint < 1.5s on a 4G simulated network. The page SHALL be instrumented with the existing `@vercel/otel` setup, naming the trace `portal.dashboard`. Web Vitals (LCP, INP, CLS) SHALL be reported to the existing observability backend.

#### Scenario: LCP budget is enforced in CI

- **WHEN** the Lighthouse CI run executes against the staging dashboard with seeded data
- **THEN** LCP is below 1.5s and CLS is below 0.05; otherwise the workflow fails
