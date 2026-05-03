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

## Documentation Rule

Every phase implementation that changes how the platform is enabled must update `docs/platform-enablement.md` in the same change. Phase-specific runbooks should link from that guide rather than replacing it.
