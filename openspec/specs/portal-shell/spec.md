# portal-shell Specification

## Purpose

The persistent app shell wrapping every authenticated Portal route — branded
sidebar, top bar with breadcrumb and command-palette trigger, tenant picker,
theme menu, language pill, notifications stream, toast rail, and route-level
permission guards. Created by archiving change `forge-portal-rebranding`.

## Requirements

### Requirement: Application shell composition

The Portal SHALL render a persistent app shell on every authenticated route consisting of (a) a fixed-width sidebar (`var(--sidebar-w)` = 248px expanded, 64px collapsed) on the left, (b) a sticky top bar of height `var(--topbar-h)` = 52px, and (c) a scrollable main canvas. The shell SHALL use CSS Grid with `grid-template-areas: "side top" "side main"` so the sidebar spans the full viewport height. The shell SHALL not flash unstyled content on first paint and SHALL preserve scroll position per route.

#### Scenario: Shell grid layout on first paint

- **WHEN** an authenticated user navigates to `/`
- **THEN** the served HTML contains `<div class="app">` with three children: `<aside class="side">`, `<header class="top">`, `<main class="main">`
- **AND** computed `grid-template-columns` is `248px 1fr` and `grid-template-rows` is `52px 1fr`

#### Scenario: Unauthenticated user is redirected to sign-in

- **WHEN** the user lacks a NextAuth session and requests any route except `/api/auth/*`
- **THEN** the server redirects to `/api/auth/signin` before the shell is rendered (no flash of shell with empty data)

### Requirement: Branded sidebar with grouped navigation

The sidebar SHALL display, from top to bottom: (i) brand header — Forge mark + the words "Forge" (Instrument Serif italic) and "Engineering Fabric" (mono eyebrow) + tenant pill showing the active tenant slug; (ii) grouped navigation with sections **Plataforma**, **Gobierno**, **Observabilidad**, **Cuenta**, each containing the corresponding routes from the brand notebook (`Tablero`, `Agentes`, `Skills`, `Herramientas MCP`, `Workflows`, `Aprobaciones`, `Specs (OpenSpec)`, `Políticas (OPA)`, `Auditoría`, `Métricas y trazas`, `Ajustes`); (iii) footer with the current user's avatar + name + role label + an overflow menu + a collapse toggle. Each `side-link` SHALL render an icon, the label and an optional pill count (e.g. agents = 14, skills = 47, mcp = 22, approvals = pending count). The active route SHALL show a 2.5px ember bar on the left edge.

#### Scenario: Active route shows ember left-bar

- **WHEN** the user is on `/approvals`
- **THEN** the `Aprobaciones` link has class `side-link active` and its `::before` pseudo-element renders a 2.5px ember-coloured bar pinned to the left edge

#### Scenario: Pill counts reflect live data

- **WHEN** the sidebar mounts
- **THEN** the `Aprobaciones` link's count comes from `GET /v1/approvals?approver=<me>&status=pending` (Set-Cookie session principal)
- **AND** the `Agentes` count comes from `GET /v1/registry/assets?kind=agent&status=approved`
- **AND** the `Skills` count comes from `GET /v1/registry/assets?kind=skill&status=approved`
- **AND** the `Herramientas MCP` count comes from `GET /v1/registry/assets?kind=mcp&status=approved`

#### Scenario: Tenant pill shows active tenant

- **WHEN** the user's active tenant is `acme`
- **THEN** the sidebar header pill renders `<button class="tenant"><svg /> acme</button>`
- **AND** clicking it opens the tenant picker popover

### Requirement: Top bar with breadcrumb, search trigger and account controls

The top bar SHALL contain (left → right): breadcrumb `Workspace · <Engineering> / <current page>`, a search trigger button that displays the platform-correct keyboard hint (`⌘K` on macOS, `Ctrl K` elsewhere) and opens the global command palette on click, an ES/EN pill, a theme menu button (sun/moon/system glyph reflecting effective theme), a notifications button with an ember dot when unread audit events exist, and a GitHub link to the active Workspace's repo.

#### Scenario: Keyboard hint reflects the user's platform

- **WHEN** `navigator.platform` matches `/mac/i`
- **THEN** the top-bar search displays the kbd hint `⌘K`
- **AND** on Windows/Linux it displays `Ctrl K`

#### Scenario: Clicking the search bar opens the command palette

- **WHEN** the user clicks anywhere on the `.top-search` element
- **THEN** the command palette opens and focus moves to its input

#### Scenario: Notifications dot reflects unread audit events

- **WHEN** the SSE stream from `/api/notifications/stream` delivers any event for the current principal
- **THEN** the notifications icon-button shows the ember `.dot`
- **AND** opening the notifications popover and acknowledging clears the dot via `POST /api/notifications/ack`

### Requirement: Theme menu

The top bar SHALL host a theme menu (button with current effective theme glyph) that opens a popover with three items: **Claro / Light**, **Oscuro / Dark**, **Sistema / System** with hint "Sigue al SO" / "Follows OS". The current preference SHALL be indicated with a check glyph. Selecting an item SHALL update the preference, fire a toast confirming the change, and update `<html data-theme>` per the design-system contract.

#### Scenario: Selecting system follows OS preference

- **WHEN** the user picks "Sistema"
- **THEN** `localStorage.forge_theme === 'system'`, the `<html>` `data-theme` matches `prefers-color-scheme`, the toast "Tema actualizado" / "Theme updated" appears for 3 seconds, and a `portal.theme.changed.v1` audit event is emitted

### Requirement: ES/EN language pill

The top bar SHALL host an ES/EN pill exposing two `aria-pressed` buttons. The active language SHALL be visually selected (ember background in dark, ink background in light) and SHALL update `<html lang>`, the persisted preference, and trigger an immediate re-render of all i18n strings.

#### Scenario: Switching to EN updates html lang attribute

- **WHEN** the user clicks the EN button
- **THEN** `<html lang="en">` is set, the sidebar section "Plataforma" becomes "Platform", the dashboard headline switches to its English copy, a toast "Language switched to English" appears, and `portal.lang.changed.v1` is audited

### Requirement: Tenant + workspace picker

The sidebar tenant pill SHALL open a Radix Popover that lists tenants the user belongs to (sourced from the `tenants` source of `/api/command-palette/search`) and, within each tenant, the workspaces (`workspaces` source). Selecting a workspace SHALL set it active, update the breadcrumb, persist the choice via `POST /api/workspace/active`, and revalidate any data on the current route via Next.js `router.refresh()`.

#### Scenario: Workspace switch revalidates the current view

- **WHEN** the user picks workspace `payments-platform` from the tenant popover while on `/runs`
- **THEN** the breadcrumb updates to `Workspace · Payments Platform / Tablero`, the runs query refetches with `tenant=acme&workspace=payments-platform`, and the audit emits `portal.workspace.switched`

### Requirement: Persistent toast / notification rail

The shell SHALL host a toast rail for transient feedback (`success`, `info`, `warn`, `error`) with 3-second auto-dismiss, 5-toast cap and 500ms in-window deduplication. Toasts SHALL be triggered by a `useToast()` hook available to every component and by the SSE stream `/api/notifications/stream` for approval/deploy/policy/run events that affect the current user.

#### Scenario: SSE event surfaces as a toast

- **WHEN** the audit stream publishes `approvals.granted.v1` for a request created by the current user
- **THEN** a `success` toast appears with the localised approval message and an optional link to `/approvals/<id>`

### Requirement: Route-level capability and permission guards

The shell SHALL hide sidebar items the user is not authorised to see (e.g. `Políticas (OPA)`, `Auditoría`, `Kill switch` for non-admin principals) based on `GET /api/permissions/me`. Navigating directly to a forbidden route SHALL render a 403 view inside the shell, not break the layout or redirect to home.

#### Scenario: Non-admin does not see the policies link

- **WHEN** the principal lacks the `policy:read` permission per OpenFGA
- **THEN** the sidebar omits the `Políticas (OPA)` link

#### Scenario: Direct visit to forbidden route renders 403 inside shell

- **WHEN** the same principal visits a forbidden route
- **THEN** the shell is rendered around a 403 card with explanation and a "Solicitar acceso" button that opens a request form

### Requirement: Collapsible sidebar and persistence

The sidebar SHALL be collapsible to a 64px icon-rail via an explicit toggle button in the side footer. The collapsed/expanded state SHALL be persisted in `localStorage` under `forge_sidebar` and applied on next mount. Viewports between 720px and 1024px wide SHALL auto-collapse regardless of the persisted preference.

#### Scenario: Sidebar collapse persists across sessions

- **WHEN** the user collapses the sidebar
- **THEN** `localStorage.forge_sidebar === 'collapsed'`, `<div class="app app--collapsed">` is applied, and reopening the Portal in a new tab keeps the icon-rail state

#### Scenario: Tablet width auto-collapses sidebar

- **WHEN** the viewport is 900px wide
- **THEN** the sidebar renders in collapsed icon-rail mode by default
- **AND** the breadcrumb in the top bar still shows the active route

### Requirement: All shell strings are i18n keys

Every visible label, eyebrow, aria-label, breadcrumb and toast message inside the shell SHALL come from the i18n dictionary; no hard-coded English or Spanish strings SHALL appear in shell component source.

#### Scenario: Audit shows no hard-coded strings

- **WHEN** `rg -n "[A-Z][a-z]+\\s+[a-z]+" portal/src/components/shell/` runs after the rebrand merges
- **THEN** the matches are limited to type names, comments and identifiers — no user-facing literal strings
