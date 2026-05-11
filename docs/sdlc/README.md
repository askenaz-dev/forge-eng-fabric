# SDLC Orchestration

Phase 4 coordinates initiatives across product, architecture, design, development, QA, security, DevOps, SRE, and FinOps.

## Core Concepts

- Initiative: one root OpenSpec plus phase state, gates, blockers, Jira, and cost context.
- Phase progression: a phase advances only when gates pass or a `phase-progression-bypass` override is approved.
- Traceability: generated artifacts link back to OpenSpec through the traceability service and OpenSpec linked artifacts.

## Services

- `services/sdlc-orchestrator`: initiative state machine, gates, blockers, and SDLC events.
- `services/traceability`: artifact graph ingestion, materialized graph query, and backfill.
- `services/mcp`: Jira and Confluence tools with Workspace-scoped enforcement.
- `services/finops`: cloud and LLM cost attribution by initiative.

## Portal

The Portal `Initiatives` module reads initiative status from `services/sdlc-orchestrator`,
traceability from `services/traceability`, and cost summaries from `services/finops`.
It does not ship seeded initiative cards or synthetic phase data; empty states mean the
underlying service returned no records for the selected Workspace.
