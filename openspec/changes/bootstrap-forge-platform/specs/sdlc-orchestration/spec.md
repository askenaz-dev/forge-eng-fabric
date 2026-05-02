## ADDED Requirements

### Requirement: Phase-aware capabilities
The platform SHALL provide agentic capabilities mapped to SDLC phases: **Product, Architecture, Design (UI/UX), Development, QA, Security, DevOps, SRE, FinOps**. Capabilities MAY be implemented as Skills, Agents or Workflows registered in the Asset Registry.

#### Scenario: QA capability generates test cases from OpenSpec
- **WHEN** an OpenSpec is updated with new acceptance criteria
- **THEN** the QA capability proposes/updates test cases linked to the OpenSpec, subject to Workspace policy

### Requirement: Coordination by Alfred
Alfred SHALL coordinate phase-aware capabilities — invoking them directly or via workflows — and SHALL consolidate their outputs into the related OpenSpec and linked artifacts.

#### Scenario: Alfred coordinates Architecture and Security for a critical change
- **WHEN** an OpenSpec is classified as security-impacting
- **THEN** Alfred invokes the Architecture capability for diagrams/ADRs and the Security capability for threat modeling, and links results to the OpenSpec

### Requirement: Traceability from intent to deploy
The orchestration SHALL preserve traceability across phases: intent → OpenSpec → backlog (Jira) → design (Figma/Confluence) → code (GitHub) → CI/CD → deploy → observability → incidents/postmortems.

#### Scenario: Trace path is queryable end-to-end
- **WHEN** a user opens a deployment in the Portal
- **THEN** the platform exposes the full trace path back to the originating intent and OpenSpec

### Requirement: Approved assets only in production-relevant phases
Phases that affect production-relevant artifacts (Architecture, Security, DevOps/SRE) SHALL invoke only `approved` assets, except in T0 sandboxes/labs explicitly authorized.

#### Scenario: Non-approved Security asset cannot be invoked in prod-related flow
- **WHEN** a Security asset in `in_review` is invoked from a flow targeting prod-related artifacts
- **THEN** the invocation is blocked and audited

### Requirement: Jira and Confluence integration
The platform SHALL integrate with **Jira** (read/write epics, stories, tasks, sprints, statuses) and **Confluence** (read/write pages, ADRs, runbooks). Integrations SHALL respect Workspace permissions and propagate identity.

#### Scenario: PO capability creates epic and stories from OpenSpec
- **WHEN** the PO capability is invoked on an OpenSpec
- **THEN** it creates/updates the corresponding Jira epic and stories, links them in the OpenSpec, and emits audit events

### Requirement: Quality and security gates as part of orchestration
The orchestration SHALL include quality and security gates (lint, unit tests, SAST, SCA, DAST, performance/e2e where applicable, evals for agentic outputs) before progressing to deploy stages, with results visible in PRs and dashboards.

#### Scenario: Failing gate blocks progression
- **WHEN** SAST or SCA fails on a PR with high-severity findings
- **THEN** the orchestration blocks progression to staging/prod stages and notifies owners
