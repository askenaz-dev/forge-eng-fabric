## ADDED Requirements

### Requirement: Alfred requires explicit App scope to create or modify specs

Every Alfred dialogue turn that could create or mutate an OpenSpec SHALL carry an explicit `app_id` in the request context. Alfred SHALL refuse to call the OpenSpec backbone with an unscoped intent and SHALL surface the App scope step (via the Intent Capture Wizard) when the caller has not yet picked one. The `app_id` SHALL be included in every decision-log entry produced during that turn.

#### Scenario: Dialogue without app_id is rejected

- **WHEN** any caller calls `POST /v1/intent/start` without `app_id` in the request body
- **THEN** Alfred MUST return `422 missing_app_scope` together with a hint pointing at the wizard's App scope step
- **AND** Alfred MUST NOT create a draft OpenSpec

#### Scenario: Decision log records app_id on every turn

- **WHEN** Alfred completes an intent dialogue turn for `app_id=app-1`
- **THEN** every decision-log entry produced for that turn MUST include `app_id=app-1`
- **AND** the audit event MUST include `app_id=app-1`

### Requirement: Alfred RAG queries are scoped to the App by default

When Alfred performs RAG retrieval inside an active dialogue with a resolved `app_id`, the query SHALL be scoped to the App's corpus first (its specs, decision logs, deployments and dashboards) and SHALL fall back to the parent Workspace corpus only when the App-scoped result set is empty or when the user explicitly broadens the scope. The retrieval log SHALL record the effective scope.

#### Scenario: App-scoped retrieval

- **GIVEN** an active dialogue for `app_id=app-1` in `workspace=ws-1`
- **WHEN** Alfred performs RAG retrieval
- **THEN** the retrieval log MUST record `scope=app:app-1` and the returned chunks MUST all belong to App `app-1`
- **AND** the retrieval MUST NOT include chunks from other Apps in `ws-1`

#### Scenario: Fallback to workspace scope when app corpus is empty

- **GIVEN** an active dialogue for a newly created App with no prior corpus
- **WHEN** Alfred performs RAG retrieval and the App corpus returns zero hits
- **THEN** the retrieval MUST automatically broaden to `scope=workspace:ws-1` and the log MUST record both the initial and the broadened scope
