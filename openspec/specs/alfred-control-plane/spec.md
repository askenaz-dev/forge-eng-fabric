# alfred-control-plane Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Alfred is the platform Control Plane Agent
The platform SHALL provide a single Control Plane Agent named **Alfred**. Alfred SHALL be able to interpret natural-language intent, consult its knowledge base, evaluate policies, create or update OpenSpecs, invoke MCPs/Skills/Workflows/Prompt Templates, delegate to specialized agents, and execute actions on integrated systems on behalf of authorized users, apps or teams.

#### Scenario: Alfred receives intent and produces an OpenSpec
- **WHEN** an authorized user expresses an intent in natural language inside a Workspace
- **THEN** Alfred parses the intent, retrieves relevant context via RAG, and creates or updates an OpenSpec linked to the Workspace, recording the action in the audit trail

### Requirement: Autonomy by default within delegated permissions
Alfred SHALL operate **autonomously by default** for actions allowed by policy and within active delegated permissions. For **human-triggered** sessions the relevant policy is the Workspace/OpenSpec `autonomy_policy`. For **non-human-triggered** sessions (`actor = system:alfred`, `trigger_source ∈ {symptom, playbook}`) the relevant policy is `policies/alfred/risk-classifier.rego`; the override lattice ensures workspace settings can only narrow OPA's decision, never relax it. Restrictions and approvals SHALL only apply when explicitly required.

#### Scenario: Autonomous action proceeds without HITL
- **WHEN** an action's effective policy decision (workspace policy for human triggers, OPA for non-human triggers) is `autonomous`/`allow` and Alfred has the necessary delegated permission
- **THEN** Alfred executes the action and records the policy decision, the `policy_bundle_hash` (when OPA-evaluated), and the outcome

#### Scenario: Action requiring approval is paused
- **WHEN** an action requires approval per the effective policy
- **THEN** Alfred opens an approval request including the action descriptor and the policy decision, halts execution, and resumes only after the configured approver(s) decide

#### Scenario: Self-protection denial overrides any allow

- **WHEN** any policy evaluation would otherwise allow an action but the target is on the self-protection denylist
- **THEN** the action SHALL be denied; admin approval SHALL NOT lift the denial; the rejection SHALL be audited as a security-channel event

### Requirement: Delegated permissions are explicit, scoped, auditable and revocable
Alfred's elevated permissions on Workspaces, repositories, clusters, pipelines, cloud projects (including federated projects) or tools SHALL be granted explicitly by an authorized owner, scoped to a target, recorded in audit, and revocable at any time.

#### Scenario: Owner revokes Alfred's elevated permission
- **WHEN** a Workspace owner revokes a previously granted elevated permission
- **THEN** Alfred can no longer perform the corresponding action on that scope and the revocation is audited

### Requirement: Decision and tool-call log
Every relevant Alfred action SHALL produce a decision record including: input intent, retrieved context references, evaluated policy, selected tool/MCP/Skill, parameters (with sensitive fields redacted), outcome and downstream effects.

#### Scenario: Tool call is fully traced
- **WHEN** Alfred invokes an MCP tool to create a GitHub repository
- **THEN** the decision log records the OpenSpec link, policy evaluated, MCP server, parameters (redacted), GitHub response and audit event with `correlation_id`

### Requirement: Alfred uses LiteLLM for all model access
Alfred SHALL access language models exclusively through the **LiteLLM** gateway. Direct provider SDK calls bypassing LiteLLM SHALL be rejected by platform policy.

#### Scenario: Model call goes through LiteLLM
- **WHEN** Alfred needs to call a language model
- **THEN** the request is routed through LiteLLM with cost tracking, fallback, and data-classification policies applied

### Requirement: Alfred RAG knowledge base on Milvus
Alfred SHALL maintain a knowledge base backed by **Milvus** that ingests OpenSpecs, runbooks, ADRs, technical documentation, repositories, PR history, workflows, Registry assets, incidents/postmortems and SDLC Team policies. RAG retrieval SHALL respect Workspace isolation and data classification.

#### Scenario: RAG retrieval respects Workspace boundary
- **WHEN** Alfred performs RAG retrieval inside Workspace A
- **THEN** the retrieved chunks belong only to sources visible to Workspace A according to OpenFGA and visibility settings

### Requirement: Delegation to specialized agents
Alfred MAY delegate sub-tasks to specialized agents registered in the Asset Registry. Delegation SHALL be policy-checked, audited and traced.

#### Scenario: Alfred delegates threat modeling to a Security Agent
- **WHEN** an OpenSpec is classified as security-impacting and a Security Agent is `approved`
- **THEN** Alfred delegates, supervises completion, consolidates results into the OpenSpec and audits the delegation### Requirement: Alfred is the Control Plane Agent
The platform SHALL run a Python+FastAPI service named **Alfred** that interprets natural-language intent, consults the knowledge base, evaluates policies, executes tools (MCPs, Skills, Prompt Templates), invokes LLMs exclusively via LiteLLM, and SHALL be able to delegate to specialized agents.

#### Scenario: Alfred turns intent into an OpenSpec
- **WHEN** an authorized user submits an intent in a Workspace
- **THEN** Alfred retrieves context via RAG, evaluates policy, creates or updates an OpenSpec linked to the Workspace, and emits a complete audit record

### Requirement: Decision log for every relevant action
Alfred SHALL produce a structured decision record for every relevant action including: input intent, retrieved context refs, evaluated policy, selected tool/MCP/Skill, parameters (sensitive fields redacted), outcome and downstream effects.

#### Scenario: Tool call is fully traced
- **WHEN** Alfred invokes a GitHub MCP tool to open a PR
- **THEN** the decision log records OpenSpec link, policy evaluated, MCP server, parameters (redacted), GitHub response and a `correlation_id`-tagged audit event

### Requirement: Alfred uses LiteLLM exclusively
Alfred SHALL access LLMs only through LiteLLM. Direct provider calls SHALL be rejected by network and platform policy.

#### Scenario: Direct provider call is denied
- **WHEN** any code path within Alfred attempts to reach a provider endpoint directly
- **THEN** the call is denied at network level and an audit event is emitted

### Requirement: RAG knowledge base on Milvus with Workspace isolation
Alfred SHALL maintain a RAG knowledge base on **Milvus** ingesting OpenSpecs, runbooks, ADRs, technical documentation, repositories, PR history, workflows, Registry assets, incidents/postmortems and SDLC Team policies. Retrieval SHALL respect Workspace isolation and data classification.

#### Scenario: Retrieval inside Workspace A excludes Workspace B sources
- **WHEN** Alfred performs RAG inside Workspace A
- **THEN** the retrieved chunks contain only sources visible to Workspace A according to OpenFGA and visibility settings

### Requirement: Wizard-driven dialogue mode

Alfred SHALL expose a dialogue API consumed by the Portal's Intent Capture Wizard, allowing a non-technical user to author an OpenSpec through a guided conversation. The dialogue surface SHALL coexist with the existing slash-command interface; neither SHALL be deprecated by this change.

#### Scenario: Wizard starts a dialogue against Alfred

- **WHEN** the Portal calls `POST /v1/intent/start` with a free-text intent
- **THEN** Alfred SHALL create a draft OpenSpec scoped to the caller's Workspace, return a `draft_id` plus the first follow-up question, and emit an `intent.dialogue.started.v1` audit event with the caller's principal

#### Scenario: Wizard answer advances the dialogue

- **WHEN** the Portal calls `POST /v1/intent/answer` with a `draft_id` and the user's answer
- **THEN** Alfred SHALL extract structured fields, persist them on the draft, decide on the next question or signal completion, and return the updated state
- **AND** Alfred SHALL emit an `intent.dialogue.turn.v1` audit event with the field changes and any LLM call ID via LiteLLM

#### Scenario: Wizard commits the draft

- **WHEN** the Portal calls `POST /v1/intent/commit` with a complete and validated `draft_id`
- **THEN** Alfred SHALL transition the draft to `committed` atomically, return the canonical `openspec_id`, and emit `intent.committed.v1` for the SDLC orchestrator to consume

### Requirement: Workspace autonomy presets surfaced by Alfred

Alfred SHALL expose Workspace-scoped autonomy presets (named bundles of `autonomy_policy`) via a read API consumed by the Portal, with a per-action-class ceiling that bounds user-level overrides.

#### Scenario: Wizard lists available presets

- **WHEN** the Portal calls `GET /v1/workspaces/{id}/autonomy/presets`
- **THEN** Alfred SHALL return the configured presets, mark one as the Workspace default, and include the per-action-class ceilings

#### Scenario: User override within ceiling accepted

- **WHEN** an answer or explicit toggle modifies an autonomy field within the ceiling
- **THEN** Alfred SHALL accept the modification and persist it on the draft

#### Scenario: User override beyond ceiling rejected

- **WHEN** an attempted override would exceed the ceiling (e.g., enable autonomous prod deploy when the ceiling forbids it)
- **THEN** Alfred SHALL reject the modification with a structured error, and Alfred SHALL emit an `autonomy.override.rejected.v1` audit event

### Requirement: Admin-managed autonomy preset configuration

Workspace admins SHALL be able to author, edit, and disable autonomy presets through a documented administrative surface. Preset changes SHALL be audited and SHALL not affect already-committed OpenSpecs retroactively.

#### Scenario: Admin authors a new preset

- **WHEN** a Workspace admin calls the preset write API with a new named preset
- **THEN** the preset SHALL be persisted, audited with the admin's principal, and made available to wizard sessions started after the change

#### Scenario: Already-committed OpenSpecs unaffected

- **WHEN** an admin disables a preset that some committed OpenSpecs reference
- **THEN** the disable SHALL only prevent new OpenSpecs from selecting it; existing OpenSpecs SHALL continue to enforce their embedded `autonomy_policy`

### Requirement: Agent-mode session lives alongside the single-intent loop

Alfred SHALL expose an **agent-mode session** surface (long-running, plan-driven, resumable) in addition to the existing single-intent loop and wizard dialogue. The single-intent loop (`POST /v1/intents`) and wizard dialogue API (`/v1/intent/start|answer|commit`) SHALL remain unchanged and SHALL continue to be the entry points for ad-hoc requests and OpenSpec authoring.

#### Scenario: Single-intent loop is unaffected by agent-mode

- **WHEN** a client posts to `POST /v1/intents` after agent-mode is deployed
- **THEN** Alfred SHALL execute exactly one loop iteration set (RAG → LLM → policy → tool/approval/final) and return an `IntentResponse` as before — no agent-mode session is created and no agent-mode events are emitted

#### Scenario: Wizard dialogue remains the OpenSpec authoring path

- **WHEN** a client drives the wizard via `/v1/intent/start|answer|commit`
- **THEN** the dialogue completes by producing a committed `openspec_id` exactly as today
- **AND** the committed OpenSpec is what an agent-mode session subsequently consumes when the user chooses to ship it

### Requirement: Per-step reuse of the existing policy and permission stack

Each agent-mode step that calls a tool, MCP or sub-agent SHALL reuse the existing per-iteration stack: delegated-permissions check via `PermissionsClient`, OPA evaluation via `PolicyClient`, redacted decision record via `Store.append_decision`, and observability capture via `AIObserver`. Agent-mode SHALL NOT introduce a parallel policy path.

#### Scenario: Step inherits the same permission gate as a single-intent call

- **WHEN** an agent-mode step invokes `mcp:github.open_pr`
- **THEN** the permission check SHALL use `subject="alfred"`, `action_class="github:write"`, `scope_kind="workspace"`, `scope_id=<workspace_id>` and the resolved criticality from the plan step — identical to a single-intent invocation of the same tool

#### Scenario: Policy denial halts the session, not just the step

- **WHEN** OPA returns `deny` for an agent-mode step
- **THEN** the session SHALL transition to `failed` with the policy rationale persisted on the session row and a final `alfred.agent_mode.failed.v1` event emitted
- **AND** subsequent steps SHALL NOT execute even if their isolated policy decision would have been `allow`

### Requirement: Resolved autonomy policy is frozen at session start

A session SHALL freeze a copy of the workspace's active `autonomy_policy` (and its per-action ceilings) on the session row at creation time. Subsequent admin edits to the workspace's presets SHALL NOT alter the running session's ceilings.

#### Scenario: Admin tightens preset after session start

- **WHEN** a workspace admin replaces `full-autonomy` with `manual-prod` while a session is running
- **THEN** the in-flight session SHALL continue under the originally frozen `full-autonomy` ceilings
- **AND** the next agent-mode session started afterward SHALL pick up the new `manual-prod` ceilings

### Requirement: Session storage and durable event log

The Alfred store SHALL persist `alfred_agent_session` rows and `alfred_agent_step` rows, both keyed by the session's `correlation_id`. Every state transition and step outcome SHALL also write a `DecisionRecord` so the audit trail and the agent-mode event log share one source of truth.

#### Scenario: Session and decisions are joined by correlation

- **WHEN** an auditor queries decisions for an agent-mode session
- **THEN** all `DecisionRecord` rows for the session SHALL share the same `correlation_id` and SHALL be retrievable via `GET /v1/agent-mode/sessions/{id}/decisions`

#### Scenario: Step row preserves dispatched workflow / tool identity

- **WHEN** a `workflow` step triggers `forge.reference.intent-to-deploy@1`
- **THEN** the `alfred_agent_step` row SHALL record the workflow id and run id, and joining back via the run id SHALL return the workflow runtime's per-step events without re-deriving them

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

### Requirement: Alfred runs dedup retrieval before persisting a new draft

For every new intent capture flow (Friendly view conversation, Advanced view `/forge` invocation, wizard, dock empty-state), Alfred SHALL run the dedup retrieval (`POST /v1/intent/match`) before persisting a draft spec. If the top hit's score meets the tenant threshold, the dialogue API SHALL return a `spec_match` block carrying the candidate(s) so the consuming UI can render the match dialog. The caller MUST send `bypass_match=true` to skip the pass (used by "Crear nuevo" and by automation that has already resolved the question).

#### Scenario: Match block returned on intent start

- **GIVEN** a workspace `ws-1` with `app-1` and a similar existing spec `spec-7`
- **WHEN** any UI calls `POST /v1/intent/start` with `{workspace_id: ws-1, app_id: app-1, intent: "Necesito una app para registrar vacaciones"}`
- **THEN** Alfred MUST first run the dedup retrieval, and if the top hit scores ≥ threshold MUST return `200 OK` with a body containing `spec_match: { candidate: { spec_id: spec-7, score, lifecycle_state, ... }, threshold }` and no `draft_id`
- **AND** the response MUST NOT persist a draft yet
- **AND** an `alfred.intent.match_found.v1` event MUST be emitted

#### Scenario: bypass_match skips dedup

- **WHEN** the UI calls `POST /v1/intent/start` with `bypass_match=true`
- **THEN** Alfred MUST skip the dedup retrieval, persist a fresh draft and return the `draft_id` + first question

#### Scenario: resume_spec_id extends an existing spec

- **WHEN** the UI calls `POST /v1/intent/start` with `resume_spec_id=spec-7`
- **THEN** Alfred MUST hydrate the existing spec into the dialogue context, persist a continuation draft referencing `spec-7`, and proceed with a continuation prompt instead of the fresh-intent prompt

### Requirement: Dialogue context carries view marker for persona rendering

Alfred's dialogue API SHALL accept a `view: "friendly" | "advanced"` field on `POST /v1/intent/start` and on `POST /v1/intent/answer`. The view marker SHALL be propagated to the LLM call so the persona rendering (label vs raw ID, citation footer style) matches the consumer surface. Audit events SHALL include `view` for slice-and-dice.

#### Scenario: Friendly view triggers label-only rendering

- **WHEN** any call carries `view=friendly`
- **THEN** Alfred's response MUST replace every raw entity ID in the rendered text with the entity's human label (using the platform label resolver)
- **AND** the audit event MUST include `view=friendly`

#### Scenario: Advanced view preserves raw IDs

- **WHEN** any call carries `view=advanced`
- **THEN** Alfred's response MAY include raw IDs in the rendered text
- **AND** the audit event MUST include `view=advanced`

### Requirement: Non-human triggers bind autonomy to the risk-classifier policy

When Alfred acts without a human in the loop (`actor = system:alfred`, `trigger_source ∈ {symptom, playbook}`), every mutating action SHALL be authorised by `policies/alfred/risk-classifier.rego`. The session executor SHALL NOT consult workspace `autonomy_policy` directly for non-human-triggered actions; instead it SHALL pass the action descriptor to OPA and obey the returned `autonomy_decision` and `sandbox_min_tier`.

#### Scenario: Symptom-triggered restart consults OPA

- **WHEN** a symptom-triggered session reaches a `mutate-runtime` step targeting a single service
- **THEN** the executor SHALL evaluate the action through `risk-classifier.rego` and, on `allow`, proceed; on `requires_approval`, pause for HITL
- **AND** SHALL ignore any conflicting workspace-level autonomy preset (the workspace can only narrow, never relax — enforced by override lattice)

#### Scenario: Workspace override narrows the OPA decision

- **WHEN** OPA returns `allow` for a non-human-triggered action but the workspace has an override declaring `requires_approval` for that action class
- **THEN** the executor SHALL apply the stricter decision (`requires_approval`)

### Requirement: Self-protection denylist binds the platform agent

Alfred SHALL NOT perform any action whose target resolves to `alfred`, `symptom-triager`, `platform-ops`, `opa`, or `keycloak`, regardless of trigger source or admin approval. The denylist is enforced by `policies/alfred/self-protection.rego` at the top of the policy chain.

#### Scenario: Alfred cannot restart its own dependency

- **WHEN** a session attempts to invoke `POST /v1/services/keycloak/restart`
- **THEN** OPA SHALL deny via self-protection
- **AND** the step SHALL be marked `failed` with `reason:"self_protection_denylist"` and the session SHALL pause for HITL

### Requirement: Session sub-principal scopes capabilities per session

For every non-human-triggered session, the triager SHALL mint a sub-principal `system:alfred:session:<uuid>` in OpenFGA whose granted capabilities are the **intersection** of `system:alfred`'s standing grants and the capabilities justified by the originating symptom's `policy_hints`. The session executor SHALL act with this sub-principal, not with `system:alfred` directly.

#### Scenario: Sub-principal restricts to symptom-relevant capabilities

- **WHEN** the symptom carries `policy_hints=["service-down"]` for `workflow-registry`
- **THEN** the minted sub-principal SHALL grant only restart/inspect capabilities scoped to that service
- **AND** SHALL NOT grant `mutate-data` or `mutate-code` regardless of standing grants

#### Scenario: Sub-principal revoked on session close

- **WHEN** a session reaches terminal status (`completed`, `failed`, `aborted`, `resolved_externally`)
- **THEN** the sub-principal SHALL be revoked from OpenFGA within 60 seconds
- **AND** the revocation SHALL be audited
