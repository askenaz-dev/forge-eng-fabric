## Context

**Forge — Engineering Fabric** es la plataforma corporativa de Agentic SDLC ("From Intent to Infrastructure"). El estado actual de la organización presenta nueve gaps clave (deployment, governance, traceability, security, quality, operational, adoption, collaboration, coordination) en la adopción de IA generativa para ingeniería: alta velocidad individual, baja entrega organizacional gobernada y trazable.

Este documento define **cómo** se implementa la propuesta — la arquitectura, contratos, decisiones técnicas, riesgos y plan de migración — manteniendo la **visión completa end-to-end** desde la Fase 0. La ejecución se realiza en fases incrementales (0–6), pero los planos lógicos, contratos y políticas quedan establecidos desde el bootstrap para evitar deuda arquitectónica.

**Stakeholders**: Business, Product, PM, Scrum Masters, UI/UX, Solution Architects/Tech Leads, Developers, QA, Security/Ethical Hackers, DevOps/SRE, Operations y el **SDLC Team** como CoE custodio.

**Restricciones**:
- Nube base **GCP**; runtimes soportados **GKE**, **Cloud Run**, **Minikube**.
- Datos sensibles tratados desde el diseño; tenancy con aislamiento por Workspace.
- Acceso a LLMs **únicamente** vía LiteLLM.
- OpenSpec como contrato vivo obligatorio para cambios relevantes.
- Todo el código generado/operado debe ser auditable y trazable a intención.

## Goals / Non-Goals

**Goals:**
- Establecer los **8 planos lógicos** (Experience, Workspace, Control, Orchestration, Agentic, Asset, Integration, Governance) con APIs y eventos definidos.
- Hacer operativos **Alfred**, el **AI Asset Registry**, **OpenSpec**, el **Custom Portal** y los runtimes soportados al cierre del roadmap.
- Garantizar **autonomía completa por defecto** para Alfred dentro de permisos delegados, con HITL configurable por policy.
- Consolidar **Security by Design**, **Observability First**, **Spec-Driven Everything** y **Asset Governance** como propiedades transversales.
- Habilitar **reutilización de assets** y **trazabilidad** intent → PR → deploy → observe → heal → evolve.
- Soportar **BYO Runtime** y **Provisioned by Forge** sobre proyectos cloud propios o federados.
- Definir el **operating model** (SDLC Team, RACI, trust levels T0–T5) y los **KPIs prioritarios** desde el inicio.

**Non-Goals:**
- No se implementa GitLab como SCM en este bootstrap (futuro).
- No se soportan runtimes fuera de GKE/Cloud Run/Minikube en la versión inicial.
- No se hace fine-tuning propio de modelos; se enruta vía LiteLLM a proveedores aprobados.
- No se construye un marketplace público externo (solo marketplace interno por tenant).
- No se reemplaza Backstage como base del Portal (se construye Custom Portal).
- No se sustituyen GitHub/Jira/Confluence; se integran y se conectan a la trazabilidad OpenSpec.

## Decisions

### D1 — Plataforma integral end-to-end vs MVP acotado
**Decisión**: Forge se especifica como plataforma integral de Agentic SDLC desde el día uno; la entrega es incremental por capacidades (Fases 0–6).
**Alternativas**: (a) MVP acotado a una fase del SDLC; (b) Adopción de Backstage + plugins.
**Rationale**: Un MVP acotado replicaría los gaps actuales y generaría deuda arquitectónica. Backstage no cubre la experiencia agéntica end-to-end ni el contrato vivo basado en OpenSpec. Definir los planos completos desde el inicio permite incrementos coherentes.

### D2 — Custom Agentic SDLC Portal (no Backstage)
**Decisión**: Construir Portal custom con **Next.js + React + Tailwind/shadcn**.
**Alternativas**: Backstage; Port; Cortex; SaaS IDP.
**Rationale**: Forge requiere experiencia agéntica conversacional (Alfred Console), Approvals Inbox, OpenSpec editor, Asset Registry, Workflow visual editor y Observability integrados. Backstage está optimizado para developer portals tradicionales y su modelo de plugins introduce fricción para una UX agéntica nativa.

### D3 — Alfred como Control Plane Agent con autonomía por defecto
**Decisión**: Alfred es **el** agente principal y opera autónomamente dentro de permisos delegados; HITL se activa solo por policy explícita ("Autonomy by Default, Policy by Exception").
**Alternativas**: Multi-agent obligatorio; HITL siempre por defecto.
**Rationale**: La autonomía maximiza velocidad y aprovecha la IA. Los riesgos se mitigan con permisos delegados acotados, scopes, audit trail, evals, guardrails y aprobaciones configurables — no con bloqueos globales por defecto. Agentes especializados pueden coexistir y ser delegados por Alfred cuando aporten valor.

### D4 — OpenSpec como contrato vivo (backbone)
**Decisión**: OpenSpec es la columna vertebral; **todo cambio relevante** se origina o queda referenciado allí. Editable por Alfred y por humanos autorizados.
**Alternativas**: Jira como única fuente; Confluence; ADRs sueltos.
**Rationale**: Jira/Confluence no expresan intención + decisiones + trazabilidad como contrato versionable y consumible por agentes. OpenSpec se vincula bidireccionalmente con esos sistemas sin reemplazarlos.

### D5 — AI Asset Registry desde el día uno
**Decisión**: Catálogo gobernado para 5 tipos: MCP, Skill, Agent, Workflow, Prompt Template, con metadata, versionado SemVer, lifecycle (proposed → in_review → approved → deprecated → retired), trust levels T0–T5 y eval scores.
**Alternativas**: Repositorios sueltos por equipo; wiki manual.
**Rationale**: Sin Registry no hay reutilización ni gobierno; es habilitador de los KPIs prioritarios (reutilización, adopción) y de seguridad de supply chain.

### D6 — LiteLLM como Model Gateway único
**Decisión**: **Todo** acceso a modelos LLM (internos y externos) pasa por LiteLLM.
**Alternativas**: SDK directo por proveedor; gateway custom.
**Rationale**: Centraliza cost tracking, fallback, rate limits, budgets por tenant/Workspace, políticas por data classification, observabilidad y revocación de proveedores; reduce vendor lock-in.

### D7 — Milvus para knowledge base de Alfred (RAG)
**Decisión**: Milvus como vector DB para RAG sobre OpenSpecs, runbooks, ADRs, repos, PRs, workflows, assets, postmortems y políticas.
**Alternativas**: pgvector; Pinecone; Weaviate.
**Rationale**: Milvus ofrece escalabilidad, ecosistema y autohospedaje en GCP/GKE, alineado con el stack on-prem-friendly y con los volúmenes esperados.

### D8 — Kafka + CloudEvents como event backbone
**Decisión**: Kafka habilitado desde el inicio como backbone async; eventos modelados con **CloudEvents**.
**Alternativas**: Pub/Sub solo; webhooks puntuales; NATS.
**Rationale**: Forge requiere eventos durables, replay, fan-out a múltiples consumidores (audit, observability, workflows, self-healing). CloudEvents estandariza contratos y permite portabilidad.

### D9 — Stack backend dual: Go (control) + Python/FastAPI (agentic)
**Decisión**: Go para Control Plane (APIs core, Registry, auth proxy, operators de Kubernetes); Python + FastAPI para Agentic Plane (Alfred, MCPs, eval harness, RAG, prompt mgmt).
**Alternativas**: Todo Python; todo Go; Java/Kotlin.
**Rationale**: Go aporta performance y operabilidad para operators, gateways y APIs de alta concurrencia. Python tiene el ecosistema dominante para IA, agentes, evals y RAG (ADK, LiteLLM, frameworks de prompting). La separación por plano evita acoplar runtimes y facilita ownership por equipo.

### D10 — Workflow Engine con editor visual + Temporal opcional
**Decisión**: Editor visual estilo n8n/Flowise en el Portal; runners aislados; **Temporal** disponible como motor de ejecución durable cuando aplique (workflows largos, checkpointing crítico).
**Alternativas**: Solo Temporal; Argo Workflows; n8n self-hosted as-is.
**Rationale**: La UX visual es requisito explícito; muchos workflows agénticos son cortos y no requieren durabilidad pesada. Temporal cubre el caso de workflows largos sin imponerse a todos. Se evalúa reutilizar capacidades existentes para no reconstruir UI desde cero.

### D11 — Keycloak (AuthN) + OpenFGA (AuthZ ReBAC)
**Decisión**: Keycloak para OIDC/SAML federado con IdP corporativo; OpenFGA para autorización fina ReBAC/Zanzibar-style.
**Alternativas**: Auth0; Casbin; OPA-only.
**Rationale**: Las relaciones Tenant → BU → Workspace → {assets, repos, ambientes} y los permisos delegados de Alfred requieren ReBAC. OpenFGA modela esto naturalmente; Keycloak cubre autenticación corporativa estándar.

### D12 — Runtimes soportados: GKE + Cloud Run + Minikube
**Decisión**: Soportar los tres runtimes con dos modos: **BYO Runtime** y **Provisioned by Forge** (vía Terraform/Config Connector/Helm). Minikube cubre local/lab/demos/servidores privados.
**Alternativas**: Solo GKE; añadir EKS/AKS; serverless puro.
**Rationale**: GCP es la nube base. GKE para apps con estado/control fino; Cloud Run para servicios stateless rápidos; Minikube para soberanía local y entornos privados. EKS/AKS quedan para futuro.

### D13 — Permisos delegados elevados sobre proyectos federados
**Decisión**: Alfred puede recibir permisos elevados explícitos, auditables, revocables y acotados por Workspace/app/ambiente sobre proyectos cloud federados (iniciativas/equipos).
**Alternativas**: Service account global; sin federación.
**Rationale**: Los proyectos destino frecuentemente pertenecen a otros equipos; la federación con scopes mínimos es más segura que un SA global y respeta ownership.

### D14 — Security by Design transversal
**Decisión**: Clasificación de datos, masking/redaction, prompt-injection defense, supply chain (SBOM con Syft, firma con Cosign), SAST/SCA/DAST/quality gates obligatorios, threat model específico de Forge (Alfred fuera de policy, MCP comprometido, prompt injection desde Confluence/Jira/GitHub, memory poisoning, supply chain de assets, etc.).
**Alternativas**: Hardening reactivo post-MVP.
**Rationale**: El daño potencial de un agente con autonomía y permisos delegados sin controles es inaceptable. La seguridad se diseña desde el bootstrap, no se atornilla después.

### D15 — Self-Healing con niveles configurables
**Decisión**: Cinco niveles operativos — Notify, Suggest, Act-with-approval, Act-autonomously, Act-and-rollback — configurables por policy/Workspace/criticidad.
**Alternativas**: On/off; siempre manual.
**Rationale**: Distintas acciones tienen distintos perfiles de riesgo. La granularidad permite madurar adopción sin sacrificar seguridad.

### D16 — Marketplace interno por tenant (no público)
**Decisión**: Los assets se comparten dentro de la **misma organización** respetando visibilidad/ownership/trust level/políticas.
**Alternativas**: Marketplace público externo; cerrado por Workspace.
**Rationale**: Maximiza reutilización organizacional sin exponer IP ni introducir riesgos de supply chain externo.

### D17 — KPIs prioritarios desde el inicio
**Decisión**: Tres KPIs prioritarios: **adopción por equipos**, **tiempo intent → PR/deploy**, **reutilización de assets**.
**Alternativas**: Solo métricas operativas; solo costo.
**Rationale**: Reflejan directamente el valor de plataforma; los demás KPIs (calidad, seguridad, costo, MTTR, NPS) son complementarios.

### D18 — SDLC Team como CoE
**Decisión**: Un equipo dedicado opera y gobierna Forge como Center of Excellence de Agentic SDLC, con potestad de aprobar cambios críticos en core/assets y de fijar políticas.
**Alternativas**: Gobierno distribuido; comité.
**Rationale**: Sin custodia clara, las plataformas pierden coherencia, seguridad y dirección. Un CoE acelera la madurez y la consistencia.

## Risks / Trade-offs

| Riesgo | Mitigación |
|---|---|
| **Autonomía excesiva mal gobernada** → acciones dañinas | Permisos delegados acotados, scopes, policies, audit trail inmutable, approvals configurables, dry-runs y rollback automático. |
| **Alfred ejecuta acción incorrecta** | Policy checks pre-ejecución, evals continuas, dry-runs, HITL configurable, observabilidad y rollback. |
| **MCP vulnerable o malicioso** | Registry lifecycle con security review, sandboxing, trust levels, allowlists, eval suite obligatoria para tools sensibles. |
| **Prompt injection desde Confluence/Jira/GitHub** | Guardrails, sanitización de RAG, separación instrucciones-sistema vs contexto externo, allowlists para tools sensibles, detección de instrucciones maliciosas. |
| **Exposición de secretos** | Secret brokering, redaction en prompts/logs, prohibición de secrets en prompts, Vault/Secret Manager, rotación, auditoría. |
| **Memory poisoning en Milvus/RAG** | Procedencia firmada de fuentes, validación, evals de retrieval, segmentación por Workspace y trust level. |
| **Costos LLM elevados** | LiteLLM con budgets por tenant/Workspace, model routing por cost class, caching de prompts, cost dashboards y alertas. |
| **Bajo adoption rate** | Registry desde día uno, UX custom, KPIs prioritarios visibles, onboarding por Workspaces, marketplace interno, evangelización SDLC Team. |
| **Duplicación de assets** | Marketplace interno con búsqueda, ownership, métricas de reutilización, eval scores y deprecation. |
| **Falta de trazabilidad** | OpenSpec obligatorio para cambios relevantes, audit trail inmutable en Kafka, vínculos PR/deploy/incident. |
| **Complejidad de permisos federados** | OpenFGA con templates IAM por Workspace/app/ambiente, revocación explícita, revisión periódica del SDLC Team. |
| **Vendor lock-in (LLM/cloud)** | LiteLLM, MCP, A2A, OpenAPI, CloudEvents, abstracciones de runtime, IaC portable. |
| **Over-engineering inicial** | Entrega incremental por fases con criterios de salida; reuse de capacidades existentes; foco en KPIs prioritarios. |
| **Escalada de permisos entre Workspaces** | Tenant/Workspace isolation por OpenFGA, scopes obligatorios, ningún permiso global por defecto. |
| **Supply chain de assets/imágenes** | Syft (SBOM), Cosign/Sigstore (firma), Trivy/Snyk (SCA), policy enforcement con OPA/Kyverno. |
| **DAST/SAST falsos positivos saturando pipelines** | Calibración por criticidad/asset, exenciones owned y auditadas, quality gates por capa. |

## Migration Plan

Como bootstrap, **no hay migración desde un sistema previo**; el plan es de **rollout por fases** con criterios de salida explícitos:

1. **Fase 0 — Foundations**: tenancy, IAM (Keycloak + OpenFGA), Postgres/Redis, Kafka, Milvus, LiteLLM, Portal base, Registry mínimo, GitHub integration, audit trail, observabilidad base. **Criterio de salida**: crear Workspace, registrar asset, conectar GitHub, autenticar Alfred y ejecutar acción simple auditada; todo acceso LLM vía LiteLLM.
2. **Fase 1 — Agentic Platform Core**: Alfred operativo (tool execution, RAG, decision log), OpenSpec CRUD, MCPs/Skills/Prompts iniciales, policies básicas. **Criterio de salida**: Alfred crea/edita OpenSpec e invoca assets aprobados con auditoría completa.
3. **Fase 2 — Workspace & App Onboarding**: creación/configuración de repos GitHub, templates, scaffolding, asociación a Workspace, owners y pipelines básicos. **Criterio de salida**: usuario crea app desde Workspace; Alfred genera scaffold y abre PR trazado a OpenSpec.
4. **Fase 3 — Deployable Apps**: registro de GKE/Cloud Run/Minikube, BYO Runtime, Provisioned by Forge, federación, IaC, deploy en ambientes bajos, observabilidad por despliegue. **Criterio de salida**: Alfred despliega app en los tres runtimes; despliegues auditados visibles en Portal.
5. **Fase 4 — SDLC Orchestration**: capacidades por fase (PO, Design, Architecture, Dev, QA, Security, DevOps, SRE) e integración Jira/Confluence. **Criterio de salida**: flujo intent-to-deploy multi-fase con trazabilidad y assets aprobados.
6. **Fase 5 — Workflow Automation & Marketplace**: editor visual avanzado, runners escalables, versionado de workflows, marketplace interno, evals avanzadas, métricas de reutilización. **Criterio de salida**: equipos publican y reutilizan assets/workflows entre Workspaces.
7. **Fase 6 — Autonomous Operations & Evolution**: self-healing, incident response asistido, postmortems automáticos, feedback de producción a OpenSpec, optimización de costos, evolution loop. **Criterio de salida**: Alfred detecta/diagnostica/corrige fallas permitidas por policy y los incidentes alimentan la knowledge base.

**Rollback strategy**:
- Cada fase es **aditiva** y desplegable con feature flags por Workspace.
- Cambios en infraestructura compartida (Kafka, Milvus, Postgres) se versionan con migraciones reversibles.
- Aprobaciones de cambios críticos custodiadas por SDLC Team; capacidad de revocar permisos delegados de Alfred en caliente.
- Audit trail en Kafka permite reconstruir y diagnosticar cualquier acción para revertir efectos colaterales.

## Open Questions

1. **Editor visual de workflows**: ¿forkear/embeber n8n o Flowise vs construir editor propio? Decisión definitiva al inicio de Fase 5; bootstrap habilita la API y el modelo de DSL.
2. **Durabilidad de workflows**: criterios concretos para usar Temporal vs runners propios (umbral de duración, checkpoint frequency, criticidad).
3. **Estrategia de evals**: harness propio vs adopción de framework externo (Langfuse evals, Promptfoo, otros) — definir en Fase 1.
4. **Modelo de costos para tenants/Workspaces**: showback vs chargeback inicial — definir con FinOps.
5. **Política de retención** de audit trail, trazas Langfuse y datos de RAG por clasificación de datos.
6. **Federación con IdP corporativo**: claims, grupos y mapping a OpenFGA tuples — definir en Fase 0 con Security/IAM.
7. **Integración GitLab futura**: priorización tras Fase 3.
8. **Marketplace público externo**: explícitamente fuera de scope; revisar tras Fase 6 según madurez y demanda.
