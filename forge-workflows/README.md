# Forge-certified workflows

This directory contains the canonical YAML DSL definitions for the Forge
seed workflows that ship as `forge-certified`. Each subfolder owns a
single workflow and its eval suite.

| Workflow | Purpose | Owner |
|---|---|---|
| `release-train` | Multi-asset release with explicit gates and human approvals | platform-engineering |
| `scaffold-and-deploy` | App onboarding (Phase 2) + deploy to dev (Phase 3) | platform-engineering |
| `incident-response` | Triage + comms + initial mitigation skeleton | sre-platform |

The eval suites live in [`eval-suites/`](eval-suites) and are registered
via the advanced eval harness as `eval_dataset` assets.

To publish:

1. `POST /v1/workflows` (workflow-registry) — create the parent record.
2. `POST /v1/workflows/{id}/versions` — publish the YAML.
3. `POST /v1/datasets` (eval-harness-adv) — register the eval dataset.
4. `POST /v1/runs/regression` — run the suite against the new version.
5. `POST /v1/marketplace` with `visibility=forge-certified` — request certification.
