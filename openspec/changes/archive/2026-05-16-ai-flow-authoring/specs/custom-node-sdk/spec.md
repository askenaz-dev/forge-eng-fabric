## ADDED Requirements

### Requirement: Custom node manifest

A custom node SHALL declare itself via a manifest file `forge-node.yaml` at the package root. The manifest SHALL include: `id` (kebab-case, unique within the publisher), `version` (SemVer), `publisher`, `display_name`, `category` (one of `trigger`, `ai`, `action`, `logic`), `description`, `inputs` schema, `outputs` schema, `config` schema, and `permissions` (list of platform capabilities the node requires).

#### Scenario: Manifest with all required fields validates

- **GIVEN** a `forge-node.yaml` with all required fields populated and valid JSON Schema for inputs/outputs/config
- **WHEN** the SDK validator runs
- **THEN** validation MUST pass
- **AND** the manifest MUST be parseable by both the Portal (for palette rendering) and the runtime (for invocation)

#### Scenario: Missing required field rejected

- **WHEN** the manifest omits `permissions`
- **THEN** validation MUST fail with `missing_required_field: permissions`

### Requirement: Custom node renderer interface

The Portal canvas SHALL expose a TypeScript interface `FlowNodeRenderer` that a custom node implements to render itself on the React Flow canvas. The interface SHALL include: a header component, a body component, an icon, a color, and an optional property-panel component. The interface SHALL be documented in `docs/sdk/custom-nodes.md` with a complete example.

#### Scenario: Example node renders end-to-end

- **GIVEN** the example custom node shipped at `portal/src/components/flow/nodes/examples/uppercase-text/`
- **WHEN** a flow author drags it onto the canvas
- **THEN** the node MUST render with its declared icon, color, header, and body
- **AND** clicking the node MUST open the property-panel component
- **AND** saving the flow MUST persist the canonical AST step with `type: custom`, `custom_node_ref: <id>@<version>`, and the configured inputs

### Requirement: Custom node runtime contract

A custom node SHALL be invokable by `workflow-runtime` over a documented protocol. The contract SHALL specify: the invocation request (workflow context, step inputs, configured `config`), the response (typed `outputs` or a structured error), the supported invocation transports (initially: HTTP POST to a publisher-hosted endpoint declared in the manifest), and the authentication mechanism (signed JWT with workspace and step claims).

#### Scenario: Runtime invokes a custom node

- **GIVEN** a published workflow using a custom node `uppercase-text@1.0.0` registered with `endpoint: https://nodes.example.com/uppercase`
- **WHEN** the workflow runs and reaches the custom-node step
- **THEN** the runtime MUST POST to the declared endpoint with a signed JWT and the step payload
- **AND** the runtime MUST timeout after the manifest-declared `timeout` (default 30s) and retry per the manifest's `retries`

#### Scenario: Custom node response validates against outputs schema

- **WHEN** a custom node returns a payload that does not match its declared `outputs` schema
- **THEN** the runtime MUST fail the step with `custom_node_output_schema_mismatch`
- **AND** downstream steps MUST NOT execute

### Requirement: Custom node SDK status — v0 with explicit instability banner

The custom node SDK SHALL ship as version `0.x` with an explicit "API may change" banner in `docs/sdk/custom-nodes.md`. The SDK SHALL NOT be promoted to `1.0` until at least three example nodes are built end-to-end and the ingestion pipeline (publishing, signing, distribution) is delivered as a separate OpenSpec change.

#### Scenario: v0 banner present

- **WHEN** a reader opens `docs/sdk/custom-nodes.md`
- **THEN** the page MUST begin with a callout stating that the SDK is `v0.x` and that breaking changes are possible until `v1.0`
- **AND** the page MUST link to the future ingestion-pipeline OpenSpec change as `tracker: TBD`

### Requirement: Custom node ingestion pipeline out of scope here

This change SHALL NOT deliver the custom-node ingestion pipeline (publication, signing, distribution registry, marketplace integration). Custom nodes in v0 SHALL be configured via a workspace-admin form that records the manifest URL and endpoint. The full pipeline is the subject of a separate change.

#### Scenario: Workspace admin registers a custom node

- **GIVEN** a workspace admin with the `node.admin` permission
- **WHEN** the admin submits a manifest URL and endpoint via the admin form
- **THEN** the platform MUST fetch and validate the manifest
- **AND** the node MUST appear in that workspace's palette under its declared `category`
- **AND** the registration MUST NOT affect other workspaces

### Requirement: Custom node permission gating

Custom nodes SHALL declare every platform capability they need (e.g., `secret.read:<scope>`, `mcp.invoke:<asset>`, `event.emit:<topic>`). At execution time the runtime SHALL grant only the declared capabilities. Attempts to use undeclared capabilities SHALL fail and emit `guardrail.trip.v1{reason=undeclared_capability}`.

#### Scenario: Undeclared capability denied

- **GIVEN** a custom node that declares `permissions: [secret.read:public]` but attempts to read a workspace-scoped secret at runtime
- **WHEN** the read happens
- **THEN** the runtime MUST deny the operation
- **AND** the guardrail event MUST be emitted with the offending capability
