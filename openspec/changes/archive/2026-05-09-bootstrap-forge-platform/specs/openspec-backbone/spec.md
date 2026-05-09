## ADDED Requirements

### Requirement: OpenSpec is the living contract for relevant changes
Every relevant change in Forge SHALL originate from or be referenced by an **OpenSpec** document. Relevance SHALL be defined by Workspace policy (e.g., user-facing changes, architecture changes, security-impacting changes, deployments to staging/prod).

#### Scenario: Production deploy requires linked OpenSpec
- **WHEN** Alfred or a user triggers a deploy to a production environment
- **THEN** the platform requires a linked, current OpenSpec; otherwise the action is blocked with a clear message

### Requirement: OpenSpec editable by Alfred and authorized humans
OpenSpecs SHALL be editable both by Alfred (acting on behalf of authorized principals) and by humans with appropriate Workspace permissions. Every modification SHALL be versioned and attributed to actor, policy and (when applicable) source intent.

#### Scenario: Human edits OpenSpec while Alfred has an open suggestion
- **WHEN** a human edits an OpenSpec while Alfred has a pending change suggestion
- **THEN** the platform records both contributions, resolves merge conflicts via the configured strategy, and audits the actors

### Requirement: Minimum OpenSpec model
An OpenSpec SHALL include at minimum: `openspec_id`, `workspace_id`, `title`, `business_intent`, `problem_statement`, `stakeholders`, `success_metrics`, `requirements (functional + non_functional)`, `constraints`, `autonomy_policy`, `linked_artifacts (jira/github/confluence/figma/ci_cd/deployments)`, `decision_log`, and `audit (created_by, updated_by, version)`.

#### Scenario: OpenSpec missing required fields is rejected
- **WHEN** an OpenSpec is created without `business_intent` or `requirements.functional`
- **THEN** the platform rejects the operation with a validation error listing missing fields

### Requirement: Bidirectional traceability with external systems
OpenSpec SHALL maintain bidirectional links with GitHub (issues, PRs, commits), Jira (epics, stories, tasks), Confluence (pages), Figma (when used), CI/CD pipelines and deployments. Links SHALL be navigable from the Portal in both directions.

#### Scenario: Navigate from PR to OpenSpec and back
- **WHEN** a user opens a PR connected to an OpenSpec
- **THEN** the Portal displays the OpenSpec from the PR view and lists the PR among the OpenSpec's linked artifacts

### Requirement: Decision log
Every OpenSpec SHALL maintain a `decision_log` capturing key decisions with `id`, `actor`, `decision`, `timestamp` and `rationale`. Decisions made by Alfred SHALL include the policy evaluated.

#### Scenario: Alfred records architectural decision in OpenSpec
- **WHEN** Alfred selects a runtime (e.g., Cloud Run) for an app based on policy and intent
- **THEN** Alfred appends a decision record to the OpenSpec with rationale and policy reference

### Requirement: Autonomy policy embedded
Each OpenSpec SHALL include an `autonomy_policy` block declaring `default_mode` (autonomous/manual) and `approvals_required` per action class. The platform SHALL enforce this policy when Alfred operates against the OpenSpec scope.

#### Scenario: OpenSpec policy overrides Workspace default
- **WHEN** an OpenSpec declares `approvals_required` for `deploy:staging` while the Workspace default is autonomous
- **THEN** the platform requires approval for staging deploys related to that OpenSpec
