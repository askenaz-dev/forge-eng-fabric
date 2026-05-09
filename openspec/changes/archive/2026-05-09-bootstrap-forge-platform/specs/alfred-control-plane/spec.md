## ADDED Requirements

### Requirement: Alfred is the platform Control Plane Agent
The platform SHALL provide a single Control Plane Agent named **Alfred**. Alfred SHALL be able to interpret natural-language intent, consult its knowledge base, evaluate policies, create or update OpenSpecs, invoke MCPs/Skills/Workflows/Prompt Templates, delegate to specialized agents, and execute actions on integrated systems on behalf of authorized users, apps or teams.

#### Scenario: Alfred receives intent and produces an OpenSpec
- **WHEN** an authorized user expresses an intent in natural language inside a Workspace
- **THEN** Alfred parses the intent, retrieves relevant context via RAG, and creates or updates an OpenSpec linked to the Workspace, recording the action in the audit trail

### Requirement: Autonomy by default within delegated permissions
Alfred SHALL operate **autonomously by default** within the scope authorized for each Workspace, user, app, environment and target project. Restrictions, approvals and limits SHALL only apply when an explicit policy defines them. Alfred SHALL NOT have unrestricted global access.

#### Scenario: Action allowed by Workspace policy executes without HITL
- **WHEN** Alfred executes an action whose policy is `autonomous` for the current Workspace/environment
- **THEN** Alfred performs the action and emits a complete audit record

#### Scenario: Action requiring approval is blocked until approved
- **WHEN** Alfred attempts an action whose policy requires approval
- **THEN** Alfred opens an approval request, halts execution, and proceeds only when the configured approver grants it

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
Alfred MAY delegate sub-tasks to specialized agents (PO, Design, Architect, Dev, QA, Security, DevOps, SRE, FinOps) registered in the Asset Registry. The existence of specialized agents SHALL NOT be required for Alfred to act.

#### Scenario: Alfred delegates threat modeling to Security Agent
- **WHEN** Alfred receives an intent requiring a threat model and a Security Agent is registered and approved
- **THEN** Alfred delegates the sub-task, supervises completion, and consolidates results into the OpenSpec
