# tenancy-onboarding-guide Specification

## Purpose
TBD - created by archiving change platform-gaps-closure. Update Purpose after archive.
## Requirements

### Requirement: Canonical tenancy model document

The platform SHALL publish `docs/concepts/tenancy-model.md` as the canonical onboarding reference for the Tenant→Business Unit→Workspace hierarchy. The document SHALL be written in a register accessible to non-technical Product, Business, and Finance stakeholders.

#### Scenario: Document linked from the Portal

- **WHEN** a user opens the Portal's "About this Workspace" panel
- **THEN** a link SHALL navigate the user to `docs/concepts/tenancy-model.md` (or its rendered equivalent)

#### Scenario: Document linked from platform enablement

- **WHEN** an operator reads `docs/platform-enablement.md`
- **THEN** the document SHALL link to `docs/concepts/tenancy-model.md` from the introduction

### Requirement: Hierarchy diagram and cloud-analogy mapping

The document SHALL include a hierarchy diagram showing Tenant → Business Unit → Workspace and SHALL map each level to its cloud-provider analogy (Tenant ≈ Org/Billing root, BU ≈ Folder, Workspace ≈ Project for GCP).

#### Scenario: Diagram renders in published docs

- **WHEN** the docs site builds
- **THEN** the diagram SHALL be embedded as a renderable image or Mermaid block, and SHALL render in the published HTML

### Requirement: Isolation matrix

The document SHALL include an isolation matrix listing what is isolated per Workspace (and per BU when relevant), covering at minimum: Kubernetes cluster scope, OpenFGA tuples and authorization checks, LiteLLM budgets and rate limits, observability tenancy (metrics/logs/traces), policy bundles, secret stores, and audit scope.

#### Scenario: Matrix lists isolation for each axis

- **WHEN** a stakeholder reviews the matrix
- **THEN** every axis listed above SHALL have a row stating where isolation is enforced (per-Tenant / per-BU / per-Workspace / shared) and how

### Requirement: Configuration patterns

The document SHALL describe at least three deployment configuration patterns (one cluster per BU, one cluster per Workspace, shared cluster with namespaces) including the trade-offs of each in terms of isolation strength, operational cost, and complexity, and the decision criteria the platform uses to recommend each.

#### Scenario: Pattern decision criteria documented

- **WHEN** a Platform Lead consults the document to choose a pattern for a new BU
- **THEN** the document SHALL state the criteria (size of the BU, sensitivity of workloads, regulatory requirements, cost ceiling) used to choose between patterns

### Requirement: Cost model documented

The document SHALL describe the cost-allocation model — showback by default, with the chargeback open question explicitly recorded — and SHALL state who in the Tenant has visibility to which costs.

#### Scenario: Cost visibility roles documented

- **WHEN** a Tenant admin reads the cost model
- **THEN** the document SHALL state which roles (Tenant admin, BU lead, Workspace owner) see which scope of cost and which roles authorize cost-allocation changes

### Requirement: Document maintenance and ownership

The document SHALL declare an owner team and SHALL be reviewed at least semi-annually. Material changes to the tenancy model SHALL require Platform Architecture and Security review.

#### Scenario: Owner and review cadence stated

- **WHEN** the document is published
- **THEN** the front matter SHALL declare `owner: <team>`, `reviewers: [<teams>]`, `last-reviewed: <date>`, and `next-review: <date>`
