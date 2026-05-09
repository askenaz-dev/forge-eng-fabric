# governance-and-operating-model Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: SDLC Team as Center of Excellence
The platform SHALL be operated and governed by a dedicated **SDLC Team** acting as the Center of Excellence for Agentic SDLC. The SDLC Team SHALL define platform policies, approve critical changes in core and critical assets, maintain standards for security, quality and architecture, manage adoption, define playbooks/runbooks, measure KPIs and coordinate with Security, Architecture, Platform, Operations and AI areas.

#### Scenario: SDLC Team approves a core change
- **WHEN** a change classified as critical to Forge core or to a critical asset is proposed
- **THEN** the change SHALL require explicit SDLC Team approval recorded in audit before being merged or activated

### Requirement: Trust levels T0–T5 enforced
The platform SHALL enforce six trust levels (T0 Experimental, T1 Read-only, T2 Internal Write, T3 SDLC Write, T4 Infra/Deploy, T5 Critical/Core). Trust level SHALL drive review depth, eval thresholds, allowed environments and required approvers.

#### Scenario: T4 Infra/Deploy asset cannot be auto-approved
- **WHEN** an asset proposed at trust level T4 attempts to move to `approved`
- **THEN** the platform requires the configured higher-tier review (e.g., DevOps/SRE + SDLC Team) before approval

### Requirement: RACI for key activities
The platform SHALL document and enforce a RACI for at least: platform operation, approval of critical assets, asset publication, core changes, Workspace creation, elevated delegated permissions, low-environment deploys and production deploys.

#### Scenario: Production deploy follows configured RACI
- **WHEN** a production deploy is requested
- **THEN** the platform identifies Responsible/Accountable per Workspace policy, requests approvals from the configured roles, informs stakeholders and audits the result

### Requirement: Internal marketplace within the Tenant
The platform SHALL provide an internal marketplace where assets can be discovered, requested, reused and rated within the same Tenant, respecting visibility, ownership, trust level and policies. The marketplace SHALL NOT expose assets outside the Tenant.

#### Scenario: Cross-team reuse via marketplace
- **WHEN** a Workspace adopts an asset published by another Workspace within the Tenant
- **THEN** adoption metrics and reuse counters update, and ownership remains with the publishing Workspace

### Requirement: Asset visibility and ownership rules
Each asset SHALL declare visibility (`workspace` or `tenant`) and an explicit owning team. Changes to ownership or visibility SHALL be auditable and require owner authorization (and SDLC Team approval for trust levels T4–T5).

#### Scenario: Change of T5 owner requires SDLC Team approval
- **WHEN** a transfer of ownership is requested for a T5 asset
- **THEN** the transfer requires SDLC Team approval and is fully audited

### Requirement: Priority KPIs governance
Governance SHALL track and report the three priority KPIs (adoption, intent → PR/deploy, asset reuse) at least monthly, plus the complementary KPIs (quality, security, reliability, MTTR, cost, NPS), to inform platform decisions.

#### Scenario: Monthly KPI review by SDLC Team
- **WHEN** the monthly governance cycle runs
- **THEN** the SDLC Team receives the consolidated KPI report and records the resulting decisions in the platform's decision log

