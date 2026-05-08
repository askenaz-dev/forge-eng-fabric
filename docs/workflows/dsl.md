# Workflow DSL

Forge workflows are described by a single canonical AST. The YAML DSL and the
visual editor are two equivalent surfaces over that AST — round-trip between
them is lossless. This page documents the YAML form.

The schema is published at
[`pkg/workflow/schema/workflow.schema.json`](../../pkg/workflow/schema/workflow.schema.json)
and embedded into the parser. The parser, schema validator and linter live in
[`pkg/workflow/`](../../pkg/workflow).

## Document shape

```yaml
apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: refine-and-pr
  name: Refine and Open PR
  version: 1.0.0
  visibility: workspace        # private | workspace | tenant | forge-certified
  criticality: medium          # low | medium | high | critical
  owners: [platform-engineering]
  description: ...
  tags: [sdlc, refine]
  success_metric: pr_merged_within_24h
spec:
  inputs:
    - name: story
      type: string
      required: true
  outputs:
    - name: pr_url
      type: string
  steps: [...]
  on_failure: [...]
```

## Step types

| Type | Required fields | Purpose |
|---|---|---|
| `skill` | `ref` | Invoke a Skill from the registry |
| `mcp` | `ref`, `tool` | Invoke an MCP tool |
| `prompt` | `ref` | Render and run a Prompt template |
| `branch` | `branches` | Conditional fan-out |
| `loop` | `for_each` | Iterate over a collection |
| `human-in-the-loop` | `approver_role` | Pause awaiting approval |
| `sub-workflow` | `workflow_ref` | Launch a child workflow |
| `event-trigger` | `event_pattern.type` | Start the workflow on a CloudEvent |

References use `registry:<type>/<id>@<version>`. Versions MUST be SemVer;
floating tags (`latest`, `main`, `stable`) are rejected. MCP refs may use
`@read|@write|@admin` permission scopes.

## Retries and compensations

```yaml
- id: open-pr
  type: mcp
  ref: registry:mcp/github@write
  tool: create_pr
  retries:
    max: 3
    backoff: exponential   # linear | fixed | exponential
    initial_ms: 100
    max_ms: 30000
  timeout: 60s
  compensate_with: rollback-pr
```

`compensate_with` references another step (typically inside `on_failure`)
that runs in saga-reverse order if the workflow fails after this step
completed successfully.

## Human-in-the-loop steps

```yaml
- id: human-approval
  type: human-in-the-loop
  approver_role: product-owner
  timeout: 24h
  on_timeout: escalate            # fail | proceed | escalate
  escalation_role: engineering-manager
```

The Approvals Inbox entry includes the upstream step outputs and the proposed
inputs to the next step. Approvers may modify those inputs before approving;
the modification is captured in the audit log alongside the original
proposal.

## Linter rules

The linter (`pkg/workflow/lint`) enforces:

- `dangling_dep` — a `depends_on` references a non-existent step.
- `unreachable_step` — a step is never reachable from an entry node.
- `cycle_detected` — `depends_on` graph contains a cycle.
- `duplicate_step_id` — two steps share the same id.
- `floating_reference_not_allowed` — registry ref isn't pinned to SemVer.
- `unknown_asset_format` — registry ref isn't well-formed.
- `missing_ref`, `missing_tool`, `missing_approver_role` — type-specific shape.
- `type_mismatch` — input wires reference unknown inputs/step outputs.

Publication is denied if the linter returns any error-severity finding.

## Round-trip and registry

`POST /v1/workflows/{id}/versions` to the workflow registry parses, schema-
validates, lints and computes the diff against the previous version. SemVer
is enforced automatically: breaking changes (input/output removal, type
change, step removal, type change of an existing step) require a major bump
or `auto_bump=true`. Published versions are immutable; the only allowed
lifecycle transition is to `deprecated`.
