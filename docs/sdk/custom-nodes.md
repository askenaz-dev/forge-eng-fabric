# Custom Node SDK (v0)

> **API may change.** The SDK is at version `0.x`. Breaking changes are possible
> until `1.0`. Promotion to `1.0` is gated on (a) shipping the ingestion
> pipeline (publication, signing, distribution registry) as a separate OpenSpec
> change and (b) at least three example nodes built externally end-to-end.
> Tracker: TBD.

Custom nodes let workspace owners extend the canonical AI-Flow node catalog
with their own integrations. They are first-class on the canvas (drag from the
palette, configure via the property panel, dry-run, save), and the runtime
invokes them like any built-in node.

## What you build

Three artifacts per custom node:

1. **A manifest** (`forge-node.yaml`) at the package root.
2. **A renderer** — a TypeScript module that implements the `FlowNodeRenderer`
   interface.
3. **An HTTP endpoint** that the workflow runtime POSTs to at execution time.

## Manifest schema

```yaml
# forge-node.yaml
id: uppercase-text                  # kebab-case, unique within publisher
version: 1.0.0                      # SemVer; floating tags rejected
publisher: acme                     # workspace-admin scoped namespace
display_name: Uppercase Text
description: Uppercases its input string.
category: action                    # one of: trigger, ai, action, logic
inputs:
  type: object
  required: [text]
  properties:
    text: { type: string }
outputs:
  type: object
  required: [text]
  properties:
    text: { type: string }
config:
  type: object
  properties:
    locale: { type: string, default: en }
permissions: []                     # platform capabilities the node needs
endpoint:
  url: https://nodes.example.com/uppercase
  timeout: 30s                      # default 30s
  retries:
    max: 3
    backoff: exponential
```

Required fields: `id`, `version`, `publisher`, `display_name`, `category`,
`description`, `inputs`, `outputs`, `config`, `permissions`. The runtime
validator rejects manifests missing any of these.

## Renderer interface

The Portal canvas exposes a TypeScript interface that every custom node
implements:

```ts
// portal/src/components/flow/customNodeApi.ts
export interface FlowNodeRenderer {
  /** Tailwind tint class for the node accent. */
  tint: string;
  /** Optional icon (SVG component or emoji shorthand). */
  icon?: React.ReactNode;
  /** Header component — usually just the display name + a status indicator. */
  Header: React.ComponentType<{ nodeId: string }>;
  /** Body component — shown inside the node box. */
  Body: React.ComponentType<{ nodeId: string; config: Record<string, unknown> }>;
  /** Property panel — shown in the right rail when the node is selected. */
  PropertyPanel?: React.ComponentType<{
    nodeId: string;
    config: Record<string, unknown>;
    onChange: (next: Record<string, unknown>) => void;
  }>;
}
```

See the working example at
[`portal/src/components/flow/nodes/examples/uppercase-text/`](../../portal/src/components/flow/nodes/examples/uppercase-text/).

## Runtime invocation contract

When the workflow runtime executes a step of `type: custom` whose manifest's
endpoint is `https://nodes.example.com/uppercase`, it POSTs:

```http
POST /uppercase HTTP/1.1
Host: nodes.example.com
Authorization: Bearer <signed JWT>
Content-Type: application/json

{
  "workflow_id": "wf-1",
  "version": "1.0.0",
  "step_id": "uppercase-1",
  "tenant_id": "t1",
  "workspace_id": "w1",
  "inputs": { "text": "hello" },
  "config": { "locale": "en" }
}
```

The JWT is signed by the platform-ops key and carries `tenant_id`,
`workspace_id`, `step_id`, `workflow_id`, and a `nbf`/`exp` window matching
the manifest-declared timeout. The endpoint MUST:

1. Verify the JWT.
2. Return `200 OK` with a body matching the manifest's `outputs` schema:
   ```json
   { "text": "HELLO" }
   ```
3. Return a non-2xx with a JSON body `{ "error": "<reason>" }` on failure.

If the response body does not match the declared `outputs` schema, the runtime
fails the step with `custom_node_output_schema_mismatch`.

## Permissions

Custom nodes declare every platform capability they need in `permissions`. The
runtime grants only the declared set at execution time; attempts to use
undeclared capabilities fail with `guardrail.trip.v1{reason=undeclared_capability}`.

Supported capability strings (v0):

- `secret.read:<scope>` — read a secret matching `<scope>` glob.
- `mcp.invoke:<asset>` — invoke a specific MCP via mcp-gateway.
- `event.emit:<topic>` — publish to the platform event bus.

## Registration (v0)

Until the ingestion pipeline ships, workspace admins register custom nodes
through the admin form at `/admin/custom-nodes`. The form takes the manifest
URL and endpoint URL, validates the manifest, and persists the registration
per workspace. Registrations do not cross workspaces.

## Future work

- Ingestion pipeline (publishing, signing, distribution registry, marketplace
  integration) — separate OpenSpec change.
- Signed Helm-style packaging for offline distribution.
- Per-tenant custom node directories.

## See also

- [`openspec/specs/custom-node-sdk/spec.md`](../../openspec/specs/custom-node-sdk/spec.md)
- [`portal/src/components/flow/nodes/examples/uppercase-text/`](../../portal/src/components/flow/nodes/examples/uppercase-text/)
- [`services/workflow-runtime/internal/runtime/activities.go`](../../services/workflow-runtime/internal/runtime/activities.go)
  (stub `stubActivity` for `type: custom`)
