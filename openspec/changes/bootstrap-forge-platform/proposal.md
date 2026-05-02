## Why

Las organizaciones que adoptan IA generativa en ingeniería ganan velocidad individual pero pierden gobierno, trazabilidad, seguridad y capacidad de despliegue end-to-end: el código se genera rápido, pero llevar una intención hasta infraestructura productiva sigue siendo manual, fragmentado y poco auditable. **Forge — Engineering Fabric** cierra ese gap con una plataforma corporativa de Agentic SDLC ("From Intent to Infrastructure") gobernada por **Alfred** (Control Plane Agent), con **OpenSpec** como contrato vivo, un **AI Asset Registry** desde el día uno, **Workspaces** como unidad operativa y despliegue sobre GKE, Cloud Run y Minikube con permisos delegados auditables.

Esta propuesta establece el **marco de implementación completo y la hoja de ruta end-to-end** de Forge. La ejecución se realizará por fases incrementales (Fase 0 → Fase 6), pero el alcance funcional, los planos arquitectónicos y los contratos quedan definidos desde el inicio para evitar reescribir la base en cada incremento.

## What Changes

- **NUEVO**: Plataforma Forge con 8 planos lógicos (Experience, Workspace, Control, Orchestration, Agentic, Asset, Integration, Governance).
- **NUEVO**: Modelo de tenancy y dominio: Tenant → Business Unit → Workspace → {Repos, OpenSpecs, Environments, Workflows, Assets, Pipelines, Deployments, Owners, Policies, Approvals, Observability}.
- **NUEVO**: **Alfred** como Control Plane Agent con autonomía completa por defecto, dentro de permisos delegados, scopes y políticas configurables; HITL configurable por Workspace/app/ambiente/acción/criticidad.
- **NUEVO**: **AI Asset Registry** como artefacto de primera clase para 5 tipos: MCP Server, Agent Skill, Agent, Workflow, Prompt Template — con lifecycle (proposed → in_review → approved → deprecated → retired), metadata, ownership, evals, trust levels (T0–T5) y visibilidad por organización.
- **NUEVO**: **AI Workflow Engine** con editor visual (estilo n8n/Flowise) en el Portal y runners aislados/observables en la Agentic Execution Platform; Temporal como opción para ejecución durable.
- **NUEVO**: **Custom Agentic SDLC Portal** (Next.js + React + Tailwind/shadcn) — con módulos: Workspaces, Alfred Console, Asset Registry, OpenSpecs, Repositories, Environments, Deployments, Workflows, Approvals Inbox, Observability, Admin & Governance.
- **NUEVO**: **OpenSpec** como columna vertebral: contrato vivo de intención, requerimientos, decisiones y trazabilidad, editable por Alfred y humanos autorizados, vinculando GitHub/Jira/Confluence/Figma/CI-CD/Deployments.
- **NUEVO**: **LiteLLM** como AI Gateway único para todo acceso a modelos (Vertex AI, MS Foundry, externos aprobados), con cost tracking, fallback, model routing, budgets por tenant/Workspace y políticas por data classification.
- **NUEVO**: **Knowledge base de Alfred** con RAG sobre **Milvus** (specs, runbooks, ADRs, repos, PRs, workflows, assets, postmortems, políticas).
- **NUEVO**: **Agentic Execution Platform**: runners, sandboxing, secret brokering, identity propagation, policy checks, rate/cost limits, retry/checkpointing, audit trail, telemetría, evals y guardrails.
- **NUEVO**: Integraciones iniciales: **GitHub** (SCM), Jira, Confluence, OpenSpec MCP, GKE, Cloud Run, Minikube, GCP, GitHub Actions/Cloud Build, OCI/Artifact Registry, SonarQube, Semgrep/CodeQL/Trivy/Snyk/OWASP ZAP, Cosign/Syft, **Keycloak** (AuthN), **OpenFGA** (AuthZ), OpenTelemetry/Prometheus/Grafana/Loki/Tempo, Langfuse, **Kafka** (event backbone) con CloudEvents.
- **NUEVO**: Modelo de runtimes soportados: **GKE**, **Cloud Run**, **Minikube**; con **BYO Runtime** y **Provisioned by Forge** sobre proyectos cloud propios o federados; permisos delegados auditables y revocables por Workspace/app/ambiente.
- **NUEVO**: **Security by Design**: clasificación de datos, masking, redaction, prompt-injection defense, supply chain (SBOM/firma), threat model específico de Forge, evals y guardrails obligatorios para tools sensibles.
- **NUEVO**: **Self-Healing & Resilience**: niveles Notify → Suggest → Act-with-approval → Act-autonomously → Act-and-rollback; postmortems automáticos y evolution loop hacia OpenSpec.
- **NUEVO**: **Operating model**: SDLC Team como CoE responsable de gobierno, políticas, aprobaciones de cambios críticos en core/assets, RACI y trust levels.
- **NUEVO**: **KPIs prioritarios**: adopción por equipos, tiempo intent → PR/deploy, reutilización de assets; KPIs complementarios de calidad, seguridad, costo, confiabilidad, MTTR y NPS interno.
- **NUEVO**: **Roadmap por fases (0–6)**: Foundations → Agentic Platform Core → Workspace & App Onboarding → Deployable Apps → SDLC Orchestration → Workflow Automation & Marketplace → Autonomous Operations & Evolution; con criterios de salida explícitos.
- Esta propuesta es **bootstrap**: no modifica capabilities preexistentes; las introduce.

## Capabilities

### New Capabilities

- `platform-foundations`: tenancy (Tenant/BU/Workspace), IAM (Keycloak + OpenFGA), audit trail, event backbone (Kafka + CloudEvents), persistencia (PostgreSQL/Redis), observabilidad base (OTel/Prometheus/Grafana/Loki/Tempo) y bootstrap del Custom Portal.
- `workspace-management`: ciclo de vida de Workspaces (creación, configuración, owners, políticas, ambientes, repos, métricas) y modelo de policies/approvals configurables por scope.
- `alfred-control-plane`: agente principal Alfred — interpretación de intención, ejecución de tool calls, delegación, memoria, RAG sobre Milvus, autonomía por defecto, permisos delegados, audit y decision log.
- `ai-asset-registry`: catálogo gobernado de MCPs, Skills, Agents, Workflows y Prompt Templates con metadata, versionado SemVer, ownership, lifecycle, trust levels, evals y visibilidad por organización.
- `openspec-backbone`: integración profunda de OpenSpec como contrato vivo de intención/requerimientos/decisiones, editable por Alfred y humanos, con trazabilidad bidireccional a GitHub/Jira/Confluence/PRs/deployments.
- `agentic-execution-platform`: runners aislados, sandboxing, secret brokering, identity propagation, policy checks, rate/cost limits, retry/checkpointing, telemetría, eval harness y guardrails para ejecución de agentes/tools/workflows.
- `ai-workflow-engine`: editor visual estilo n8n/Flowise en el Portal, DSL declarativo, versionado, runners durables (con opción Temporal) y catálogo de nodos (LLM, MCP, Skill, Agent, Prompt Template, HITL gate, branch, loop, retry, eval, webhook, GitHub/Deploy/Approval/Notification).
- `model-gateway`: LiteLLM como punto único de acceso a modelos internos/externos con switching, fallback, cost tracking, budgets por tenant/Workspace, rate limits, políticas por data classification y observabilidad.
- `app-onboarding`: conexión GitHub-first, creación/configuración de repos, templates, scaffolding de servicios, branches/PRs, asociación a Workspace, owners y pipelines básicos.
- `deployment-platform`: soporte de **GKE**, **Cloud Run** y **Minikube** con BYO Runtime y Provisioned by Forge, IaC (Terraform/Config Connector/Helm), proyectos federados, permisos delegados elevados auditables y despliegue automatizado en ambientes bajos/altos según policy.
- `sdlc-orchestration`: capacidades agénticas por fase del SDLC (PO, Design, Architecture, Dev, QA, Security, DevOps, SRE, FinOps) coordinadas por Alfred y/o workflows, con trazabilidad completa a OpenSpec.
- `security-by-design`: data classification, masking/redaction, prompt-injection defense, supply chain (SBOM/Cosign), SAST/SCA/DAST/quality gates, threat model de Forge, secrets management y políticas de tenancy/aislamiento.
- `observability-and-metrics`: telemetría operacional, agéntica, de producto/adopción, costo y seguridad; AI observability con Langfuse; dashboards por Workspace y por asset.
- `self-healing-and-resilience`: detección/diagnóstico/acción correctiva con niveles configurables (Notify → Act-and-rollback), runbooks asistidos, postmortems automáticos y evolution loop hacia OpenSpec.
- `governance-and-operating-model`: SDLC Team (CoE), RACI, trust levels (T0–T5), aprobaciones de cambios críticos, marketplace interno y políticas de visibilidad/uso de assets.

### Modified Capabilities

<!-- Bootstrap del producto: no hay capabilities preexistentes en openspec/specs/. -->

## Impact

- **Código y servicios nuevos**:
  - Backend Control Plane en **Go** (APIs core, Registry, auth proxy, operators).
  - Backend Agentic Plane en **Python + FastAPI** (Alfred, MCPs, eval harness, RAG, prompt mgmt).
  - Frontend **Next.js + React + Tailwind/shadcn** (Portal custom).
  - Operadores de Kubernetes para GKE/Minikube y conectores Cloud Run.
  - MCP Servers iniciales (GitHub, Jira, Confluence, OpenSpec, Kubernetes/GKE, Cloud Run, Observability, Security tools, Secret Manager).
- **APIs y eventos**:
  - APIs REST/gRPC documentadas con OpenAPI por plano lógico.
  - Eventos en Kafka con esquema **CloudEvents** (Workspace, Asset, Agent, Workflow, Deployment, Audit, Incident).
  - Soporte de **MCP** y **A2A** como contratos abiertos para extensibilidad.
- **Dependencias / infraestructura**:
  - GCP como cloud base; runtimes GKE, Cloud Run, Minikube.
  - PostgreSQL/Cloud SQL, Redis/Memorystore, Milvus, Kafka, Keycloak, OpenFGA.
  - LiteLLM, Langfuse, OpenTelemetry stack, SonarQube, Semgrep/CodeQL/Trivy/Snyk/OWASP ZAP, Cosign/Syft.
  - Terraform, Config Connector, Helm, GitHub Actions/Cloud Build, Argo CD (cuando aplique).
- **Sistemas afectados / integrados**: GitHub (SCM inicial), Jira, Confluence, OpenSpec, Vertex AI / MS Foundry / proveedores LLM externos aprobados.
- **Organizacional**:
  - Conformación del **SDLC Team** como CoE con responsabilidades de gobierno, aprobación de cambios críticos en core/assets y custodia de políticas.
  - Definición de RACI inicial y trust levels T0–T5 para assets.
- **Riesgos**: autonomía mal gobernada, MCPs vulnerables, prompt injection, exposición de secretos, costos LLM, baja adopción, complejidad de permisos federados, vendor lock-in. Mitigaciones detalladas en design.md y mapeadas a controles concretos.
- **Out of scope (ahora)**: GitLab (futuro), proveedores cloud distintos a GCP en runtime base, fine-tuning propio de modelos, marketplace público externo.
