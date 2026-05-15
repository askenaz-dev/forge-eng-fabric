# Forge Workflow DSL

The Forge Workflow DSL is a YAML dialect that compiles losslessly to the canonical
workflow AST (`pkg/workflow/ast`). Both the visual editor and the CLI produce the
same AST; this package handles the YAML ↔ AST bridge.

## Quick start

```yaml
apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: my-workflow
  name: My Workflow
  version: 1.0.0
  visibility: workspace       # private | workspace | tenant | forge-certified
  criticality: medium         # low | medium | high | critical
spec:
  inputs:
    - name: app_id
      type: string
      required: true
  steps:
    - id: generate-iac
      type: skill
      ref: registry:skill/sdlc-iac/generate-terraform@1.0.0
      inputs:
        app_id: $inputs.app_id
      targets:
        iac: required
    - id: approve
      type: human-in-the-loop
      approver_role: platform-admin
      depends_on: [generate-iac]
```

## Step types

| Type | Required fields | Notes |
|------|----------------|-------|
| `skill` | `ref` | Invokes a registered skill via the skill-gateway |
| `mcp` | `ref`, `tool` | Calls an MCP server tool |
| `prompt` | `ref` | Direct LLM prompt step |
| `branch` | `branches[]` | Conditional branching |
| `loop` | `for_each`, `body` | Iteration over a list |
| `human-in-the-loop` | `approver_role` | Pauses for human approval |
| `sub-workflow` | `workflow_ref` | Embeds another workflow |
| `event-trigger` | `event_pattern` | Waits for a CloudEvent |

## `targets:` map (sdlc-end-to-end)

A step MAY carry a `targets:` map to override the App's SDLC phase policy for
that step. Keys are canonical phase names; values are one of the four allowed
policy strings.

**Canonical phase keys:** `architect` `design` `development` `qa` `security`
`devops` `iac` `sre` `finops` `observability`

**Allowed values:**

| Value | Meaning |
|-------|---------|
| `required` | Phase runs; workflow fails if any required gate fails |
| `optional` | Phase runs; gate failure emits a warning, does not fail |
| `opt-in` | Phase only runs when explicitly requested at workflow start |
| `skipped` | Phase removed from the plan entirely |

**Tightening rule:** a per-step override may only make a phase *more strict*
(e.g. `opt-in → required`) never more permissive (e.g. `required → optional`).
The orchestrator enforces this at plan-build time and rejects the run with
`409 cannot_relax_required_phase` if the rule is violated.

```yaml
steps:
  - id: run-iac
    type: skill
    ref: registry:skill/sdlc-iac/validate-iac@1.0.0
    targets:
      iac: required   # tightens App-level opt-in → required for this step
      sre: optional   # allowed only if App-level sre is not already required
```

The DSL linter (`pkg/workflow/lint`) reports `invalid_target_phase` for unknown
keys and `invalid_target_value` for unknown values as errors.

## Reference resolution

Asset refs take the form `registry:<type>/<group>/<name>@<semver>`. Floating
tags (`:latest`, `:^1`) are rejected by the linter with `floating_reference_not_allowed`.

## Round-trip guarantee

`Parse(Marshal(wf)) == wf` for all well-formed workflows. The test suite
verifies this including the new `targets:` map field.
