# Forge Engineering Fabric

Forge Engineering Fabric is the platform repo for the SDLC control plane: tenancy, IAM, audit, Registry, Alfred, OpenSpec, policies, approvals, MCPs, skills, prompts, observability, deployment orchestration and autonomous operations.

The canonical enablement guide is maintained as a living step-by-step document:

- [Platform Enablement Guide](docs/platform-enablement.md)
- [Phase 0 local getting started](docs/getting-started.md)
- [Phase 1 integrated validation runbook](docs/governance/phase-1-integrated-runbook.md)

## Current Status

| Phase | Change | Status |
|---|---|---|
| 0 | `phase-0-foundations` | Local-first foundations implemented with remaining sign-off/deferred infra items tracked in OpenSpec. |
| 1 | `phase-1-agentic-core` | Core agentic services implemented; integrated staging evidence and SDLC sign-off still pending for final exit tasks. |
| 2+ | `phase-2-app-onboarding` through `phase-6-autonomous-ops` | Proposed/planned specs exist and will extend the enablement guide as implementation proceeds. |

## First Commands

```sh
make bootstrap
make up
make ps
```

Use `docs/platform-enablement.md` before running phase-specific validation. It records required tools, environment variables, service URLs, bootstrap order, evidence collection and sign-off criteria.

## Portal

The Internal Developer Portal under `portal/` is the Forge Engineering Fabric
front-end. It ships with the **Forge** design system — Ember palette,
Instrument Serif display, Geist UI and JetBrains Mono code — driven by
CSS-variable tokens, a `light` / `dark` / `system` theme system, a
`compact` / `comfortable` / `spacious` density system, ES (default) / EN
i18n and a global ⌘K command palette. Every visible surface binds to live
platform services — no mocks ship.

- [Design system reference](docs/portal/design-system.md)
- [i18n & glossary](docs/portal/i18n.md)
- [Component inventory](docs/portal/components.md)

Run locally:

```sh
cd portal
pnpm install
PORTAL_REBRAND=1 pnpm dev
# open http://localhost:3000 — ⌘K opens the command palette
```

## Documentation Rule

Every phase implementation that changes how the platform is enabled must update `docs/platform-enablement.md` in the same change. Phase-specific runbooks should link from that guide rather than replacing it.
