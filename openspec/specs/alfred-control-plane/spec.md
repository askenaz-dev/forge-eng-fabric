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
Alfred SHALL operate **autonomously by default** for actions allowed by Workspace/OpenSpec policy and within active delegated permissions. Restrictions and approvals SHALL only apply when explicitly required.

#### Scenario: Autonomous action proceeds without HITL
- **WHEN** an action's policy is `autonomous` and Alfred has the necessary delegated permission
- **THEN** Alfred executes the action and records the policy decision and outcome

#### Scenario: Action requiring approval is paused
- **WHEN** an action requires approval per policy
- **THEN** Alfred opens an approval request, halts execution, and resumes only after the configured approver decides### Requirement: Delegated permissions are explicit, scoped, auditable and revocable
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
