# security-by-design Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Data classification and tenancy isolation
Every data element processed by the platform SHALL be classifiable (e.g., public, internal, confidential, restricted). Tenant and Workspace data SHALL be isolated at storage, processing and retrieval (RAG) levels.

#### Scenario: RAG cannot retrieve cross-Workspace confidential data
- **WHEN** Alfred performs RAG in Workspace A
- **THEN** confidential data scoped to Workspace B is never returned

### Requirement: Masking and redaction for sensitive fields
Sensitive fields (PII, secrets, tokens) SHALL be masked or redacted in prompts, tool inputs/outputs, logs and traces. Configurable redaction policies SHALL be applied consistently.

#### Scenario: Token never appears in logs or trace
- **WHEN** a tool input contains a token
- **THEN** the token is redacted in logs, OpenTelemetry attributes, Langfuse traces and decision-log records

### Requirement: Secrets management
Secrets SHALL be stored in a Secret Manager / Vault, brokered to runners just-in-time, never embedded in prompts, never logged, and rotated according to policy.

#### Scenario: Secret never embedded in prompt
- **WHEN** a Skill needs a secret to perform an external call
- **THEN** the secret is injected by the broker and is never serialized into the prompt or stored alongside the conversation

### Requirement: Prompt-injection defense
The platform SHALL apply prompt-injection defenses: separation of system instructions from external context, sanitization of RAG content, allowlists for sensitive tools, detection of malicious instructions in retrieved documents, and policy checks before tool execution.

#### Scenario: Injected instruction blocked from invoking a sensitive tool
- **WHEN** retrieved content contains text instructing the model to call a tool not in the allowlist
- **THEN** the call is blocked and the attempt is logged as a prompt-injection event

### Requirement: Supply-chain security
The platform SHALL produce SBOMs (with **Syft**), sign artifacts (with **Cosign/Sigstore**), run SCA (Trivy/Snyk) and validate dependencies. Unsigned or non-compliant artifacts SHALL NOT be deployable to higher environments.

#### Scenario: Unsigned image cannot deploy to staging/prod
- **WHEN** a deployment targets staging or prod with an unsigned image
- **THEN** the deployment is rejected by policy and the attempt is audited

### Requirement: Quality and security gates
The platform SHALL enforce: **SonarQube** quality gate, **SAST** (Semgrep/CodeQL), **SCA** (Trivy/Snyk), **DAST** (OWASP ZAP) and evals for agentic outputs. Gates SHALL be calibrated by criticality and trust level.

#### Scenario: Critical SAST finding blocks merge
- **WHEN** a PR has unresolved critical SAST findings
- **THEN** merge is blocked and owners are notified

### Requirement: Threat model addressed
The platform design SHALL explicitly address: Alfred acting outside policy, compromised MCP, vulnerable Skills, malicious Prompt Templates, exfiltration via tool output, prompt injection from Confluence/Jira/GitHub, memory poisoning in Milvus/RAG, cross-Workspace privilege escalation, misuse of external LLM providers and supply-chain risks for workflows/agents.

#### Scenario: Memory poisoning attempt is rejected at ingestion
- **WHEN** content with provenance failing validation is submitted to the RAG ingestion
- **THEN** the platform rejects ingestion, isolates the source, and audits the event

