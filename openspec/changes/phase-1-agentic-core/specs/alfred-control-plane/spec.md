## ADDED Requirements

### Requirement: Alfred is the Control Plane Agent
The platform SHALL run a Python+FastAPI service named **Alfred** that interprets natural-language intent, consults the knowledge base, evaluates policies, executes tools (MCPs, Skills, Prompt Templates), invokes LLMs exclusively via LiteLLM, and SHALL be able to delegate to specialized agents.

#### Scenario: Alfred turns intent into an OpenSpec
- **WHEN** an authorized user submits an intent in a Workspace
- **THEN** Alfred retrieves context via RAG, evaluates policy, creates or updates an OpenSpec linked to the Workspace, and emits a complete audit record

### Requirement: Autonomy by default within delegated permissions
Alfred SHALL operate **autonomously by default** for actions allowed by Workspace/OpenSpec policy and within active delegated permissions. Restrictions and approvals SHALL only apply when explicitly required.

#### Scenario: Autonomous action proceeds without HITL
- **WHEN** an action's policy is `autonomous` and Alfred has the necessary delegated permission
- **THEN** Alfred executes the action and records the policy decision and outcome

#### Scenario: Action requiring approval is paused
- **WHEN** an action requires approval per policy
- **THEN** Alfred opens an approval request, halts execution, and resumes only after the configured approver decides

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

### Requirement: Delegation to specialized agents
Alfred MAY delegate sub-tasks to specialized agents registered in the Asset Registry. Delegation SHALL be policy-checked, audited and traced.

#### Scenario: Alfred delegates threat modeling to a Security Agent
- **WHEN** an OpenSpec is classified as security-impacting and a Security Agent is `approved`
- **THEN** Alfred delegates, supervises completion, consolidates results into the OpenSpec and audits the delegation
