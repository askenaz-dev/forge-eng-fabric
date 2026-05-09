## ADDED Requirements

### Requirement: OpenSpec is the living contract
Every relevant change in Forge SHALL originate from or be referenced by an **OpenSpec**. Relevance is defined by Workspace policy.

#### Scenario: Production-relevant action without OpenSpec is blocked
- **WHEN** Alfred or a user triggers a production-relevant action without a linked OpenSpec
- **THEN** the platform blocks the action with a clear error and offers to create or link an OpenSpec

### Requirement: OpenSpec editable by Alfred and authorized humans
OpenSpecs SHALL be editable by Alfred (acting on behalf of authorized principals) and by humans with appropriate Workspace permissions. Edits SHALL be versioned and attributed.

#### Scenario: Concurrent edit produces auditable resolution
- **WHEN** a human edits an OpenSpec while Alfred has a pending suggestion
- **THEN** both contributions are recorded, conflict is resolved per configured strategy, and actors/policies are audited

### Requirement: Minimum OpenSpec model
An OpenSpec SHALL include at minimum: `openspec_id`, `workspace_id`, `title`, `business_intent`, `problem_statement`, `stakeholders`, `success_metrics`, `requirements (functional + non_functional)`, `constraints`, `autonomy_policy`, `linked_artifacts`, `decision_log`, `audit`.

#### Scenario: OpenSpec missing required fields is rejected
- **WHEN** an OpenSpec is submitted without `business_intent` or `requirements.functional`
- **THEN** the platform rejects the operation listing missing fields

### Requirement: Bidirectional traceability
OpenSpec SHALL maintain bidirectional links with GitHub (issues, PRs, commits), Jira (epics, stories, tasks), Confluence (pages), Figma (when used), CI/CD pipelines and deployments. Links SHALL be navigable in both directions from the Portal.

#### Scenario: Navigate from PR to OpenSpec and back
- **WHEN** a user opens a PR connected to an OpenSpec
- **THEN** the Portal displays the linked OpenSpec from the PR view, and the OpenSpec lists the PR among `linked_artifacts`

### Requirement: Decision log
Each OpenSpec SHALL maintain a `decision_log` with `id`, `actor`, `decision`, `timestamp` and `rationale`. Decisions made by Alfred SHALL include the policy evaluated and `correlation_id`.

#### Scenario: Alfred records a decision in the OpenSpec
- **WHEN** Alfred makes a relevant decision (e.g., choosing a runtime)
- **THEN** the decision is appended to `decision_log` with rationale and policy reference

### Requirement: Embedded autonomy policy
Each OpenSpec SHALL include an `autonomy_policy` block (`default_mode`, `approvals_required`). Policy enforcement SHALL apply when Alfred operates against the OpenSpec scope, overriding Workspace defaults when more restrictive.

#### Scenario: OpenSpec policy overrides Workspace default
- **WHEN** the OpenSpec requires approval for `deploy:staging` while the Workspace default is autonomous
- **THEN** the platform requires approval for staging deploys related to that OpenSpec
