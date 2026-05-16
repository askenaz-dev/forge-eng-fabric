---
title: Third-Party License Inventory
owner: platform-architecture
reviewers: [security, legal]
last-reviewed: 2026-05-16
next-review: 2026-11-16
---

# Third-Party License Inventory

This document tracks every third-party component embedded in or distributed with Forge Engineering Fabric, their license, the version pinned, and any contribution-back obligations. The `notes` column flags special handling (LGPL link/distribution rules, attribution requirements, etc.).

## Active inventory

| Component | License | Version | Where used | Notes |
|---|---|---|---|---|
| @xyflow/react | MIT | 12.3.5 (pinned in `portal/package.json`) | AI-Flow visual editor canvas (`portal/src/components/flow/`) | None. Adopted by [ADR-0002](./adrs/0002-canvas-react-flow.md), supersedes Flowise entry. |
| Next.js | MIT | per `portal/package.json` | Portal | None |
| FastAPI | MIT | per `services/*/pyproject.toml` | Python services | None |
| OpenFGA | Apache-2.0 | per compose | Authorization | None |
| LiteLLM | MIT | per compose | LLM gateway | None |
| Keycloak | Apache-2.0 | per compose | Identity | None |
| Loki / Tempo / Mimir / Grafana | AGPL-3.0 | per Helm values | Observability | Self-hosted; no public service exposure required |
| OpenTelemetry Collector | Apache-2.0 | per compose | Telemetry | None |
| Postgres / Kafka / Redis / Milvus | various OSI-approved | per compose | Data plane | None |

## Adding a new dependency

1. Open a PR that adds the dependency and updates this table.
2. Confirm license compatibility with the [forge-distribution policy](#distribution-policy) below.
3. If the license requires source distribution (LGPL, GPL, AGPL), add a note to this table describing how that obligation is met.
4. Reviewer set: Platform Architecture + Security + Legal.

## Distribution policy

Forge Engineering Fabric is distributed in two forms:

- **Self-hosted** by customers — no special distribution obligations beyond honouring component licenses inside the customer environment.
- **Hosted** by Forge — we may need to make source available for AGPL-licensed components if used in a way that constitutes "providing" the software to remote users. As a rule, AGPL components run only as observability infrastructure (Loki, Tempo, Grafana, Mimir) where customers are not directly interacting with the AGPL surface.

When in doubt, escalate to Legal and Security.

## Review cadence

This document is reviewed semi-annually and on every PR that adds, removes, or upgrades a third-party dependency. Material changes require co-approval from Platform Architecture, Security, and Legal.
