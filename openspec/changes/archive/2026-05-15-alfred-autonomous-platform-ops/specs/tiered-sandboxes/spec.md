## ADDED Requirements

### Requirement: Four sandbox tiers with bounded cost and lifetime

`platform-ops` SHALL expose a sandbox primitive with four tiers: L0 dry-run-in-place, L1 single-service container, L2 ephemeral Kubernetes namespace, L3 ephemeral full stack. Every spawned sandbox SHALL carry a hard TTL (default 30min) and a billing tag enabling FinOps attribution.

#### Scenario: L0 dry-run is in-process and synchronous

- **WHEN** Alfred calls `POST /v1/sandbox/spawn` with `tier:0` and `action_descriptor` referencing a migration
- **THEN** the endpoint SHALL evaluate the dry-run (e.g., `EXPLAIN` or `terraform plan`) without provisioning external resources and respond synchronously with the result

#### Scenario: L1 single-service container

- **WHEN** Alfred requests `tier:1` for a service under test
- **THEN** the endpoint SHALL run a docker container of that service against a copy-on-write database snapshot and return its endpoint URL plus a `sandbox_id`

#### Scenario: L2 ephemeral namespace

- **WHEN** Alfred requests `tier:2`
- **THEN** the endpoint SHALL provision a Kubernetes namespace cloned from `dev` with synthetic-only data, label it `forge.sandbox/owner=system:alfred` and return the kubeconfig context and namespace name

#### Scenario: TTL enforcement

- **WHEN** a sandbox reaches its TTL
- **THEN** `platform-ops` SHALL invoke `destroy` automatically and write an audit row with `outcome:"ttl_expired"`

### Requirement: OPA decides the minimum tier, Alfred may escalate

The minimum tier for any mutating action SHALL be determined by `risk-classifier.rego` (`sandbox_min_tier`). Alfred MAY request a higher tier when extra confidence is worth the cost; Alfred MUST NOT request a lower tier than the minimum.

#### Scenario: Below-minimum tier is rejected

- **WHEN** Alfred calls `POST /v1/sandbox/spawn` with `tier:0` for an action whose policy mandates `sandbox_min_tier:1`
- **THEN** the endpoint SHALL respond 403 with `code=below_min_tier`

#### Scenario: Above-minimum tier is allowed and logged

- **WHEN** Alfred requests `tier:2` for an action whose minimum is `tier:1`
- **THEN** the endpoint SHALL provision tier 2 and emit `alfred.sandbox.tier_escalated{from:1, to:2, reason:"..."}`
- **AND** the cost SHALL be attributed to the tenant the action targets

### Requirement: Sandboxes are isolated and produce a verification artifact

Every sandbox SHALL be isolated from production data and external systems (no outbound network to prod APIs, secrets are mock secrets). On `POST /v1/sandbox/{id}/run`, the platform SHALL execute the candidate action and emit a structured verification artifact (test results, diff, probe outcomes) that the calling endpoint uses as input to the verification gate.

#### Scenario: Mock secrets only

- **WHEN** a sandbox attempts to read a secret
- **THEN** it SHALL receive a mock value tagged `mock:true` and SHALL NOT be able to retrieve production credentials

#### Scenario: No outbound prod calls

- **WHEN** code running in a sandbox attempts a network call to a production URL
- **THEN** the sandbox network policy SHALL block the request and the attempt SHALL be logged

#### Scenario: Verification artifact is returned

- **WHEN** `POST /v1/sandbox/{id}/run` completes
- **THEN** the response SHALL include `{verification: {probe_results:[...], diff:..., logs_ref:"..."}}` consumable by the verification gate

### Requirement: Sandbox lifecycle is auditable

Every sandbox `spawn|run|destroy` SHALL produce its own audit row. The `sandbox_run` table SHALL link each sandbox to its triggering session, the action being validated, the policy bundle, the TTL, and the destruction reason.

#### Scenario: Audit trail per sandbox

- **WHEN** a sandbox spawns, runs, and is destroyed
- **THEN** three audit rows SHALL exist with `correlation_id` linking them to each other and to the parent session and action
