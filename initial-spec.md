# Engineering Fabric — Especificación / Specification

> **Status:** Draft v0.3
> **Tipo de documento:** Pitch ejecutivo + Anexo técnico (RFC)
> **Alcance:** Plataforma corporativa de ingeniería de software impulsada por IA generativa con capacidades agénticas end-to-end ("From Intent to Infrastructure")
> **Changelog v0.2:** decisiones resueltas sobre cloud, workflow engine, modelo de despliegue híbrido, AI Gateway, autonomía agéntica, gobernanza cross-tenant, ownership agéntico y modelo de Focals (ver §24).
> **Changelog v0.3:** decisiones operativas resueltas (Temporal self-hosted, retención 30 días, onboarding Modo B híbrido, ownership de specs por focal de fase, GitHub App por tenant) + stack tecnológico definido (polyglot Go + Python/FastAPI, Next.js, PostgreSQL, Redis, Milvus, Kafka). Ver §25.

---

## Tabla de Contenidos

**Parte I — Pitch Ejecutivo**
1. Resumen ejecutivo
2. Naming: propuestas y recomendación
3. El problema: los gaps actuales del desarrollo con IA
4. La visión: *From Intent to Infrastructure*
5. Stakeholders y propuesta de valor
6. Principios rectores (Guiding Principles)
7. Capacidades core (vista de alto nivel)
8. Outcomes esperados y KPIs
9. Roadmap de alto nivel

**Parte II — Anexo Técnico (Technical RFC)**
10. Glosario
11. Arquitectura lógica
12. Componentes core
13. Capacidades agénticas en profundidad
14. Flujo end-to-end: *From Intent to Infrastructure*
15. Integraciones
16. OpenSpec como columna vertebral
17. Security by Design
18. Self-Healing & Resilience
19. Stack tecnológico propuesto
20. Roadmap detallado por fases
21. Riesgos y mitigaciones
22. Decisiones resueltas (v0.2)
23. Referencias
24. **Adendum v0.2 — Decisiones de Producto (detalle)**
25. **Adendum v0.3 — Decisiones Operativas y Stack Tecnológico**

---

# Parte I — Pitch Ejecutivo

## 1. Resumen Ejecutivo

**Engineering Fabric** (nombre tentativo, ver §2) es una plataforma interna que **orquesta capacidades de IA generativa y agentes autónomos a lo largo de todo el ciclo de vida del software** — desde la formulación de una idea hasta su despliegue, operación y evolución en producción.

Su propósito es cerrar el gap más crítico que enfrentan hoy las organizaciones que adoptan IA en ingeniería: **la IA generativa produce código y artefactos rápidamente, pero gobernarlos, desplegarlos, monitorearlos y mantenerlos de forma segura y consistente sigue siendo manual, frágil y poco trazable.**

La plataforma combina:

- Un **Registry de AI Assets** (MCP Servers, Agent Skills, Agents) con su *how-to* y políticas de uso.
- Un **AI Workflow Engine** (al estilo n8n / Flowise) para componer pipelines agénticos.
- Un **Control Plane Agent** ("Jarvis-like") que conoce todo el ecosistema y orquesta extremo a extremo.
- Un **Developer Portal** con admin, métricas, ownership, salud y calidad.
- **Trazabilidad nativa con OpenSpec**, **Security by Design**, **Human-in-the-Loop (HITL)** configurable y **Self-Healing** operacional.

El resultado: cualquier participante del SDLC — Product Owners, Scrum Masters, Designers, Architects, Tech Leads, Developers, DevOps, Security, QA, Operaciones — puede llevar una intención a infraestructura productiva **cumpliendo estándares de calidad, seguridad, arquitectura y datos**, con la velocidad de la IA y el rigor de una organización madura.

---

## 2. Naming: Propuestas y Recomendación

El nombre debe transmitir: **(a)** integración/tejido de capacidades, **(b)** ingeniería seria, **(c)** un toque memorable. Estas son cinco alternativas evaluadas:

| # | Nombre | Metáfora / Razonamiento | Pros | Contras |
|---|--------|--------------------------|------|---------|
| 1 | **Engineering Fabric** | Tejido entrelazado de capacidades agénticas | Claro, descriptivo, suena corporativo y serio | "Fabric" está overloaded (Microsoft Fabric, Service Fabric, HashiCorp); poca distintividad |
| 2 | **Forge** *(o AI Forge / GenForge)* | La fragua donde se templa software de calidad | Corto, fuerte, evoca *crafting*; alinea con cultura de ingeniería | "Forge" usado por GitHub Forge, Laravel Forge, etc. |
| 3 | **Continuum** | El continuo *intent → code → infra → ops → evolution* sin costuras | Captura exactamente la promesa central ("intent-to-infra"); elegante | Suena un poco abstracto/marketinero |
| 4 | **Praxis** | Griego: el puente entre teoría (intent) y práctica (infra desplegada) | Único, memorable, intelectualmente atractivo | Requiere explicación inicial; puede sonar pretencioso |
| 5 | **Helix** | Doble hélice humano + IA co-evolucionando el software | Visualmente potente, sugiere evolución y *self-healing* | Microsoft tiene un producto Helix; conflicto de marca |

### Recomendación

**Top pick: `Forge`** (con sub-branding **"Forge — Engineering Fabric"** si se quiere mantener la palabra).
Razón: combina memorabilidad + cultura ingenieril + facilidad de uso conversacional (*"deployalo desde Forge"*, *"está publicado en el Forge Registry"*). El branding compuesto te da lo mejor de ambos mundos.

**Segundo lugar:** `Continuum`, si el énfasis estratégico está más en el storytelling del lifecycle completo que en la cultura de craft.

### Naming del Control Plane Agent ("el Jarvis")

Tres opciones para el agente orquestador, alineadas a la metáfora del *trusted advisor*:

- **Alfred** — directo al guiño de Batman; cálido, leal, omnisciente sobre el "manor". Ya está en tu mente, así que tiene tracción interna.
- **Daedalus** *(Dédalo)* — el maestro artesano de la mitología, constructor del laberinto; encaja perfecto con un agente que conoce toda la arquitectura.
- **Hermes** — el mensajero que conecta todos los planos; útil cuando el agente actúa como router entre sub-agentes (A2A).

**Sugerencia:** **Alfred**. El nombre ya tiene resonancia, es corto, y la analogía Batman/Alfred se vende sola en cualquier presentación interna.

> A partir de aquí el documento usa **Forge** y **Alfred** como placeholders. Reemplazables 1:1 si la decisión final cambia.

---

## 3. El Problema: Los Gaps Actuales del Desarrollo con IA

Hoy, las organizaciones que adoptan IA generativa en ingeniería viven una paradoja: **la productividad individual sube, pero la entrega organizacional no escala proporcionalmente.** Los principales gaps:

1. **Gap de despliegue (*deployment gap*).** El código generado por IA llega rápido al IDE, pero el camino a producción sigue siendo manual: pipelines, IaC, secrets, ambientes, dominios, certificados.
2. **Gap de gobernanza.** No hay catálogo único de qué MCPs, skills y agentes están aprobados, quién los mantiene, ni cómo se versionan.
3. **Gap de observabilidad y mantenimiento.** Los artefactos creados con IA rara vez tienen métricas, logging, tracing y *runbooks* de calidad productiva desde el día 1.
4. **Gap de trazabilidad de intención.** Se pierde el hilo entre *"esta línea de código"* → *"esta historia de usuario"* → *"esta decisión de producto"* → *"esta intención original"*.
5. **Gap de seguridad.** *Prompt injection*, *supply chain* de modelos y prompts, secrets filtrados a contextos LLM, agentes con permisos excesivos.
6. **Gap de calidad consistente.** Cada equipo aplica IA con su propio criterio, sin barandas comunes (SAST, OWASP, SonarQube, *eval harness*).
7. **Gap de adopción medible.** No se sabe quién usa qué, cuánto valor genera, ni dónde están los cuellos de botella.

**Forge cierra estos siete gaps.**

---

## 4. La Visión: *From Intent to Infrastructure*

> Una persona del negocio o un PO formula una intención en lenguaje natural. Un agente la traduce en *épicas, HUs y tareas* (Jira) y *especificaciones OpenSpec*. Otro agente diseña la arquitectura (con diagramas) y la UI/UX. El equipo (humano + agentes) construye, valida calidad y seguridad automáticamente, publica imágenes en el registry, despliega en GKE/Cloud Run, habilita dominio, monitorea salud y, si algo falla, **Alfred** dispara *self-healing*. Toda la cadena está documentada, auditada y trazable hasta la intención original.

Esa es Forge. El usuario humano define el *qué* y el *por qué*; la plataforma orquesta el *cómo*, manteniendo barandas de calidad, seguridad y arquitectura, con HITL donde corresponde y autonomía donde se haya habilitado.

---

## 5. Stakeholders y Propuesta de Valor

| Rol | Cómo Forge le acelera el ciclo |
|-----|--------------------------------|
| **Product Owners / Project Managers** | Convertir intención en backlog estructurado (épicas/HUs) en minutos, con OpenSpecs trazables. Forecasting asistido por IA. |
| **Scrum Masters** | Métricas de equipo en tiempo real, detección de impedimentos, automatización de ceremonias administrativas. |
| **UI/UX Designers** | Generación de wireframes, design tokens y componentes desde la HU; *design-to-code* validado. |
| **Solution Architects / Tech Leads** | Generación de diagramas (C4, secuencia, despliegue), validación de fitness functions, ADRs asistidos. |
| **Developers** | Skills + MCPs preaprobados; *scaffold* de servicios cumpliendo estándar; PRs con contexto OpenSpec. |
| **DevOps / SRE** | IaC generado con baranda, despliegues a GKE/Cloud Run, observabilidad por defecto, self-healing. |
| **Security / Ethical Hackers** | SAST/SCA/DAST integrados, *threat modeling* asistido, registro de findings con SLA. |
| **QA / Testers (funcionales y técnicos)** | Generación de casos de prueba desde OpenSpec, *test data synthesis*, regresión asistida. |
| **Operational Teams** | Runbooks autogenerados, *incident response* asistido por Alfred, postmortems con análisis causal. |

---

## 6. Principios Rectores (Guiding Principles)

1. **Security by Design** — controles desde el primer commit, no como capa final.
2. **Spec-Driven Everything** — toda intención queda capturada en **OpenSpec** y es la fuente de verdad.
3. **Human-in-the-Loop configurable** — niveles de autonomía por tipo de acción (ver §12.6).
4. **Observability First** — *no metrics, no merge*. Todo artefacto nace con telemetría.
5. **Asset Governance** — MCPs, Skills y Agents son artefactos de primera clase: versionados, owned, auditados.
6. **Composable & Open** — APIs abiertas, estándares (MCP, A2A), evita *vendor lock-in*.
7. **Documentation as Code** — la doc se genera y se mantiene desde el spec, no aparte.
8. **Measurable Adoption** — si no se mide, no existe; *adoption metrics* en cada superficie.

---

## 7. Capacidades Core (vista de alto nivel)

```
┌───────────────────────────── FORGE ─────────────────────────────┐
│                                                                 │
│   ┌──────────────────────┐    ┌────────────────────────────┐    │
│   │  Developer Portal    │◄──►│   Alfred (Control Plane)   │    │
│   │  (UI / Admin / Obs)  │    │   ─ orchestrator agent ─   │    │
│   └──────────────────────┘    └────────────────────────────┘    │
│            ▲                              ▲                     │
│            │                              │                     │
│   ┌────────┴─────────┐         ┌──────────┴─────────┐           │
│   │  AI Asset        │         │  AI Workflow       │           │
│   │  Registry        │         │  Engine            │           │
│   │  (MCP/Skills/    │         │  (n8n/Flowise-     │           │
│   │   Agents)        │         │   style)           │           │
│   └──────────────────┘         └────────────────────┘           │
│            ▲                              ▲                     │
│            └──────────────┬───────────────┘                     │
│                           │                                     │
│   ┌───────────────────────┴───────────────────────────┐         │
│   │  Plataforma de Ejecución Agéntica                 │         │
│   │  (sandboxing, secrets, IAM, telemetry, eval)      │         │
│   └───────────────────────────────────────────────────┘         │
│                           │                                     │
│   ┌───────────────────────┴───────────────────────────┐         │
│   │  Integraciones: Jira · Confluence · GitHub/GitLab │         │
│   │   · GKE · Cloud Run · Sonar · OWASP · OpenSpec    │         │
│   │   · Keycloak · OpenFGA · Observability Stack      │         │
│   └───────────────────────────────────────────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

---

## 8. Outcomes Esperados y KPIs

| Categoría | KPI | Meta a 12 meses |
|-----------|-----|------------------|
| **Velocidad** | *Lead time* intent → producción | -50 % vs baseline |
| **Adopción** | % de equipos con al menos 1 flow productivo en Forge | ≥ 70 % |
| **Calidad** | Coverage SAST/DAST en componentes generados | 100 % |
| **Seguridad** | Vulnerabilidades críticas en producción originadas en componentes Forge | 0 |
| **Confiabilidad** | MTTR para incidentes auto-recuperables | < 5 min (self-healing) |
| **Trazabilidad** | % de PRs con OpenSpec linkeado | 100 % |
| **Eficiencia AI** | Costo promedio de tokens por feature | -30 % YoY (vía caching, modelos pequeños, evals) |
| **Satisfacción** | NPS interno de la plataforma | ≥ +40 |

---

## 9. Roadmap de Alto Nivel

- **Fase 0 — Foundations (M0–M2):** IAM (Keycloak + OpenFGA), Asset Registry mínimo, Portal v0, integración con Git/Jira.
- **Fase 1 — Agentic Core (M2–M5):** Workflow Engine, Skills/MCPs aprobados, Alfred v0 (read-only), OpenSpec end-to-end.
- **Fase 2 — Intent-to-Code (M5–M8):** generación de épicas/HUs, scaffolding de servicios, design generation, PRs asistidos.
- **Fase 3 — Code-to-Infra (M8–M11):** despliegues a GKE/Cloud Run, IaC, dominios, observabilidad por defecto.
- **Fase 4 — Self-Healing & Evolution (M11–M14):** Alfred con acción autónoma controlada, *self-healing*, evolución asistida del software.
- **Fase 5 — Scale & Optimize (M14+):** multi-tenant maduro, *cost optimization*, *eval harness* avanzado, marketplace interno de assets.

> Cada fase incluye gates de seguridad, evaluación de adopción y revisión de KPIs.

---

# Parte II — Anexo Técnico (Technical RFC)

## 10. Glosario

| Término | Definición |
|---------|------------|
| **Agent Skill** | Capacidad reutilizable y autocontenida que un agente puede invocar (ver agentskills.io). |
| **MCP Server** | *Model Context Protocol* server — expone recursos/herramientas a LLMs vía protocolo estandarizado. |
| **Agent** | Entidad autónoma con un loop de razonamiento, herramientas y objetivos (ver ADK, A2A). |
| **A2A** | *Agent-to-Agent* — protocolo de comunicación inter-agentes. |
| **OpenSpec** | Formato/estándar para especificar intención y requerimientos trazables (openspec.dev). |
| **HITL** | *Human-in-the-Loop* — punto donde un humano aprueba o ajusta una acción agéntica. |
| **Control Plane Agent** | Agente meta que tiene visibilidad y orquestación sobre todo el ecosistema (Alfred). |
| **Self-Healing** | Capacidad de la plataforma de detectar fallas y recuperarse de forma autónoma. |
| **Eval Harness** | Suite de evaluación automática para medir calidad de salidas de agentes/LLMs. |

---

## 11. Arquitectura Lógica

Forge se estructura en **seis planos** lógicos:

1. **Experience Plane** — Developer Portal, CLI, IDE plugins, ChatOps.
2. **Orchestration Plane** — Alfred + Workflow Engine.
3. **Agentic Plane** — Agents, MCPs, Skills, sandboxing y eval.
4. **Asset Plane** — Registry de AI Assets + metadata, ownership, versionado.
5. **Integration Plane** — conectores a Jira, Confluence, Git, Sonar, GKE, Cloud Run, observability stack.
6. **Governance Plane** — IAM, RBAC/ReBAC, auditoría, políticas, billing, *cost control*.

> Cada plano expone APIs documentadas (OpenAPI) y eventos (CloudEvents) hacia un *event backbone* (Pub/Sub o Kafka).

---

## 12. Componentes Core

### 12.1 AI Asset Registry

Catálogo único de MCPs, Skills y Agents aprobados.

- **Modelo de metadatos:** `id`, `tipo` (mcp|skill|agent), `version` (SemVer), `owner_team`, `inputs/outputs schema`, `permisos requeridos`, `data sensitivity`, `cost class`, `eval scores`, `SLA`, `runbook URL`, `OpenSpec link`.
- **How-to aislado:** cada asset incluye un *playground* sandbox y ejemplos ejecutables.
- **Lifecycle:** `proposed → in-review → approved → deprecated → retired`.
- **Storage:** PostgreSQL para metadata, OCI registry / Artifact Registry para artefactos binarios.

### 12.2 AI Workflow Engine

Plataforma para componer flujos agénticos visual y programáticamente (estilo **n8n / Flowise**, pero con foco en agentes y MCP).

- Editor visual + DSL exportable a YAML.
- Nodos: LLM call, MCP tool, Skill invoke, Agent delegate, HITL gate, branch, loop, retry, eval, webhook.
- Versionado de workflows en Git.
- Ejecución serverless (Cloud Run jobs / Knative) con *checkpointing* para flujos largos.

### 12.3 Control Plane Agent — **Alfred**

Agente orquestador con visibilidad transversal. Inspirado en frameworks como **openclaw.ai** / ADK / A2A.

- **Capacidades:**
  - Conoce el estado de todos los componentes (vía Asset Registry + telemetría).
  - Recibe intenciones en lenguaje natural y las descompone en sub-tareas delegadas a sub-agentes especializados (PO Agent, Architect Agent, Dev Agent, QA Agent, SRE Agent…).
  - Mantiene memoria de largo plazo (vector DB + structured store) por tenant/proyecto.
  - Ejecuta *policy checks* antes de cada acción autónoma.
- **Modelo de autonomía:** configurable por tipo de acción (ver §12.6) — desde *suggest-only* hasta *act-with-rollback*.
- **Trazabilidad:** cada decisión queda en un *decision log* inmutable enlazado al OpenSpec origen.

### 12.4 Developer Portal

UI única para todos los stakeholders.

- **Módulos:**
  - **Catalog** — explorar assets y soluciones.
  - **Workflows** — crear/operar flujos agénticos.
  - **Solutions** — vista por componente: owner, salud, calidad, costos, OpenSpec.
  - **Admin** — gestión de tenants, roles, presupuestos.
  - **Observability** — dashboards de salud, calidad, seguridad y adopción.
  - **Approvals Inbox** — aprobaciones HITL pendientes.
- **Tecnología sugerida:** Backstage (CNCF) como base, con plugins propios; o stack custom con Next.js + shadcn/ui.

### 12.5 Observability & Metrics

Tres dimensiones:

1. **Operacional** — logs, traces (OpenTelemetry), métricas (Prometheus), uptime de componentes desplegados.
2. **AI/Agentic** — token usage, latency, eval scores, *hallucination rate*, *tool error rate*, *guardrail trips*.
3. **Producto/Adopción** — DAU/WAU, time-to-first-success, retention por superficie, *flows ejecutados*, *intent-to-prod cycle time*.

Stack: OpenTelemetry + Prometheus + Grafana + Loki/Tempo + Cloud Logging. Para AI ops, considerar **Langfuse** o equivalente.

### 12.6 Approval & HITL Flows

Niveles de autonomía configurables por *acción × entorno × criticidad*:

| Nivel | Comportamiento |
|-------|----------------|
| **L0 — Suggest** | El agente propone, humano ejecuta. |
| **L1 — Approve-each** | El agente ejecuta tras aprobación HITL por step. |
| **L2 — Approve-batch** | Aprobación por workflow completo, no por step. |
| **L3 — Auto-with-rollback** | Ejecución autónoma con rollback automático ante eval fail. |
| **L4 — Full Autonomous** | Ejecución autónoma sin gate (solo para acciones idempotentes y reversibles). |

Política recomendada: **prod = L1/L2**, **staging = L3**, **dev = L4**.

> **Regla de autonomía para mejoras (v0.2):** por defecto, toda propuesta de mejora generada por un agente requiere **aprobación humana** antes de mergear/desplegar, sin importar el ambiente. La autonomía de tipo L3/L4 sobre cambios de código solo se habilita si el OpenSpec del componente o la configuración del proyecto declara **explícitamente** `autonomous_improvements: enabled` con la lista de tipos de cambio permitidos (ej: dependency bumps menores, refactors de coverage, fixes de lint). Cada acción autónoma queda registrada con la cláusula del spec que la autorizó. Detalle completo en §24.5.

### 12.7 IAM & Authorization

- **Authentication:** **Keycloak** (OIDC + SAML), federado con IdP corporativo.
- **Authorization fina:** **OpenFGA** (modelo ReBAC/Zanzibar-style) para permisos granulares sobre assets, workflows, solutions, environments.
- **Modelo de relaciones:**
  ```
  user → member → team
  team → owner → solution
  solution → contains → workflow
  workflow → uses → asset
  asset → has_visibility → tenant|public|private
  ```
- **Audit trail:** todos los `check`/`write` quedan registrados con contexto.

---

## 13. Capacidades Agénticas en Profundidad

### 13.1 Agent Skills (agentskills.io)
Unidades empaquetadas de capacidad. En Forge, cada skill se publica al Registry con metadatos, permisos, eval suite y *cost class*. Los agentes solo pueden invocar skills aprobados para su rol.

### 13.2 MCP Servers (modelcontextprotocol.io)
Conectores estandarizados a fuentes de datos y herramientas (Jira, Confluence, Git, BigQuery, etc.). Forge mantiene una flota de MCP servers gestionados, con *credential brokering* vía secret manager y *audit logging* por invocación.

### 13.3 Agents (ADK + A2A)
- **ADK** (adk.dev) como framework base para construir agentes.
- **A2A** (github.com/a2aproject/A2A) como protocolo de comunicación inter-agentes.
- Patrón típico: **Alfred** (orquestador) → sub-agentes especializados (PO, Architect, Dev, QA, SRE) → MCPs/Skills.

### 13.4 Otras capacidades a considerar

- **RAG corporativo** sobre Confluence, ADRs, runbooks, código histórico.
- **Knowledge Graphs** — grafo de servicios, owners, dependencias, OpenSpecs (Neo4j o equivalente).
- **Eval Harness** — *golden datasets*, regresión de prompts, *judge LLMs*, *red teaming* automatizado.
- **Prompt Registry** — versionado de prompts/system messages con A/B testing.
- **Model Router** — selección dinámica de modelo según costo/latencia/calidad.
- **Synthetic Data Generation** — para testing y evaluación.

---

## 14. Flujo End-to-End: *From Intent to Infrastructure*

```
[Intent en lenguaje natural]
        │
        ▼
(Alfred) ── descompone ──► OpenSpec v0
        │
        ▼
[PO Agent]    → genera Épicas + HUs + criterios de aceptación → Jira
[Design Agent]→ wireframes + design tokens                   → Confluence/Figma
[Arch Agent]  → diagramas C4 + ADRs + threat model           → Confluence
        │
        ▼
[Dev Agent]   → scaffolding + código + tests                 → GitHub/GitLab PR
        │
        ▼
[QA Agent]    → tests funcionales + e2e + perf               → CI
[Sec Agent]   → SAST + SCA + DAST + OWASP checks             → Sonar/Defect Tracker
        │
        ▼
[HITL Gate (configurable)]
        │
        ▼
[Build]       → imagen OCI                                   → Artifact Registry
[Deploy]      → GKE / Cloud Run + dominios + certs           → Producción
        │
        ▼
[SRE Agent + Alfred] → observabilidad activa, alertas, self-healing
        │
        ▼
[Evolution Loop] → feedback de prod → nuevo OpenSpec → ciclo se repite
```

Cada paso queda enlazado al OpenSpec original; el grafo es navegable desde cualquier artefacto.

---

## 15. Integraciones

| Dominio | Sistema | Integración inicial |
|---------|---------|---------------------|
| Backlog / PM | **Jira** | MCP server: crear/editar épicas, HUs, sprints |
| Documentación | **Confluence** | MCP server: leer/escribir páginas; RAG indexing |
| SCM | **GitHub / GitLab** | MCP server: PRs, branches, reviews, GitHub Apps/GitLab Apps |
| CI/CD | GitHub Actions / GitLab CI / Cloud Build | Templates corporativos generados |
| Calidad | **SonarQube** | Quality gate obligatorio en pipeline |
| Seguridad | **OWASP ZAP**, Snyk/Trivy, SAST tools | Etapa obligatoria; findings al portal |
| Diagramas | Mermaid, Structurizr, PlantUML | Render server-side; export a Confluence |
| Runtime | **GKE**, **Cloud Run** (extensible a EKS/AKS) | IaC con Terraform / Config Connector |
| Observability | OpenTelemetry, Prometheus, Grafana, Cloud Ops | Auto-instrumentation por template |
| AuthN/Z | **Keycloak**, **OpenFGA** | SDK propio para apps Forge |
| Spec | **OpenSpec** (openspec.dev) | Backbone de trazabilidad |

---

## 16. OpenSpec como Columna Vertebral

OpenSpec captura **intención** de forma estructurada y trazable. En Forge, **toda acción agéntica relevante referencia un OpenSpec ID**. Esto permite responder en cualquier momento: *"¿de dónde salió este código / esta tabla / este endpoint / este deploy?"*

- **Storage:** repositorio Git dedicado por tenant + DB indexada para queries.
- **Convenciones:** un OpenSpec por épica, con *children specs* por HU y *technical specs* por servicio.
- **Linkage:** OpenSpec ID viaja en headers de PR, commits (trailer), Jira issues (custom field), deploy annotations, OpenTelemetry resources.
- **Doc generation:** la documentación final del producto se compone automáticamente desde los OpenSpecs ejecutados.

---

## 17. Security by Design

Controles transversales obligatorios:

- **Identity & Access:** Keycloak + OpenFGA, *least privilege* por defecto, *just-in-time access* para acciones privilegiadas.
- **Secret management:** GCP Secret Manager / HashiCorp Vault; nunca en prompts ni en logs.
- **Prompt injection defense:** *guardrails* en cada llamada LLM, allowlist de dominios para web tools, *content filtering* en outputs.
- **Supply chain:** firma de imágenes (Cosign/Sigstore), SBOM (Syft), policy enforcement (Kyverno / OPA).
- **Code quality gates:** SonarQube quality gate, SAST (Semgrep/CodeQL), SCA (Trivy/Snyk), DAST (OWASP ZAP).
- **Threat modeling automatizado:** Architect Agent produce STRIDE/LINDDUN inicial; Sec Team revisa.
- **Data protection:** clasificación de datos por sensibilidad; PII detection en datasets de eval; *data residency* por tenant.
- **Audit:** todo evento agéntico relevante → log inmutable (con hash chain).
- **Red teaming continuo:** Sec Agent ejecuta *adversarial prompts* contra agentes propios.
- **Compliance:** mapping a estándares (OWASP Top 10, OWASP LLM Top 10, ISO 27001, SOC 2 según aplique).

---

## 18. Self-Healing & Resilience

Alfred + SRE Agent monitorean SLOs y disparan acciones de recuperación:

- **Detección:** alertas Prometheus / Cloud Monitoring → event en Pub/Sub.
- **Diagnóstico:** SRE Agent correlaciona logs, traces, métricas; consulta runbook generado desde OpenSpec.
- **Recuperación:**
  - L0: notifica solamente.
  - L1: propone acción con HITL.
  - L2: ejecuta acción reversible (restart, rollback, scale).
  - L3: ejecuta playbook completo con post-validación.
- **Postmortem:** generación automática de borrador, con *blameless framing*; humano valida y publica.
- **Aprendizaje:** cada incidente alimenta el knowledge graph para mejorar diagnósticos futuros.

---

## 19. Stack Tecnológico Propuesto

| Capa | Tecnología sugerida |
|------|---------------------|
| Cloud base | **GCP** (GKE Autopilot + Cloud Run + Cloud SQL + Memorystore) |
| **Backend — Control Plane** | **Go** (API Gateway, Asset Registry, Auth proxy, Workflow Engine glue, K8s operators) |
| **Backend — Agentic Plane** | **Python + FastAPI** (Agents, MCP servers, eval harness, RAG pipelines, prompt management) |
| **Frontend** | **Next.js + React** (+ shadcn/ui + Tailwind) — Portal y consolas admin |
| **Base de datos relacional** | **PostgreSQL** (Cloud SQL en GCP); esquema por bounded context |
| **Cache** | **Redis** (Memorystore en GCP, modo cluster para HA) |
| **Vector DB** | **Milvus** (https://milvus.io/) — self-hosted en GKE |
| **Event backbone** | **Apache Kafka** (self-hosted vía Strimzi en GKE) para streaming/agent events; Pub/Sub opcional para integraciones GCP-native |
| API gateway externo | Apigee o Cloud Endpoints |
| Workflow engine | **Temporal self-hosted** (ver §25.2) con capa visual propia (estilo n8n/Flowise) |
| Agent framework | **ADK** (adk.dev) + A2A protocol |
| Control plane agent | basado en **openclaw.ai** o framework equivalente |
| MCP servers | Stack oficial MCP + servers propios (Python/FastAPI) |
| LLM providers | **Vertex AI** (GCP) y **Microsoft Foundry** (Azure) como proveedores base |
| **AI Gateway** | **LiteLLM** (litellm.ai) — único punto de acceso a LLMs, con switching, fallback, cost tracking y guardrails uniformes |
| Knowledge graph | Neo4j o Spanner Graph |
| Auth | **Keycloak** + **OpenFGA** |
| Observability | OpenTelemetry + Prometheus + Grafana + Tempo + Loki + **Langfuse** |
| Calidad/Seguridad | SonarQube, Semgrep, Trivy, OWASP ZAP, Cosign, Syft |
| IaC | Terraform + Config Connector + Helm |
| CI/CD | GitHub Actions / GitLab CI + Cloud Build + Argo CD |
| Spec | **OpenSpec** (openspec.dev) |

> **Decisión confirmada (v0.2):** Workflow Engine **build sobre Temporal**, con una capa visual propia inspirada en la UX de n8n/Flowise para que los workflows sean accesibles a no-developers. Razón: durabilidad, checkpointing y ejecución de flujos largos (días/semanas) son críticos para agentes enterprise, capacidades que n8n/Flowise no cubren a escala. Detalle en §24.2.

---

## 20. Roadmap Detallado por Fases

### Fase 0 — Foundations (M0–M2)
- Keycloak + OpenFGA productivos.
- Asset Registry v0 (CRUD + metadata).
- Portal v0 (catalog read-only).
- Integraciones base: GitHub, Jira (MCP servers).
- OpenSpec repos + convenciones.
- **Gate:** revisión de seguridad y arquitectura.

### Fase 1 — Agentic Core (M2–M5)
- Workflow Engine v1 sobre Temporal.
- Skills/MCPs aprobados (kit inicial: 10 skills, 5 MCPs).
- Alfred v0 (read-only: responde preguntas sobre el ecosistema).
- Eval harness mínimo.
- Telemetría AI (Langfuse).
- **Gate:** primer flujo end-to-end intent→PR (sin deploy aún).

### Fase 2 — Intent-to-Code (M5–M8)
- PO Agent (épicas/HUs en Jira).
- Design Agent (wireframes, design tokens).
- Architect Agent (C4 + ADR + threat model inicial).
- Dev Agent (scaffolding + tests).
- HITL inbox en Portal.
- **Gate:** 3 equipos piloto con flujo intent→PR aprobado.

### Fase 3 — Code-to-Infra (M8–M11)
- Pipelines templated (Sonar + SAST + SCA + DAST obligatorios).
- Deploy a Cloud Run y GKE.
- Habilitación de dominio + certs automatizada.
- Observability por defecto en cada deploy.
- **Gate:** primer servicio productivo nacido 100% en Forge.

### Fase 4 — Self-Healing & Evolution (M11–M14)
- SRE Agent con playbooks autoejecutables (L2/L3).
- Postmortems asistidos.
- Evolution loop: feedback de prod → nuevos specs.
- Alfred con autonomía L2 en staging y L1 en prod.
- **Gate:** reducción medible de MTTR e incidentes.

### Fase 5 — Scale & Optimize (M14+)
- Multi-tenant maduro (isolation + billing por tenant).
- Marketplace interno de assets (con ratings y rev-share simbólico).
- Cost optimization avanzado (model routing, prompt caching).
- Red teaming continuo + bug bounty interno.

---

## 21. Riesgos y Mitigaciones

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Sobre-autonomía agéntica causa incidentes | Alto | Defaults conservadores (L0/L1 en prod), rollback automático, eval harness obligatorio. |
| Costos LLM se disparan | Medio | Model router, prompt caching, presupuestos por tenant, alertas de spend. |
| Vendor lock-in con un LLM provider | Medio | Multi-provider desde día 1, abstracción vía Model Router. |
| Prompt injection / data exfiltration | Alto | Guardrails, allowlists, *output scanning*, red teaming continuo. |
| Adopción baja por curva de aprendizaje | Alto | Plantillas pre-armadas, playgrounds, champions por equipo, onboarding guiado por Alfred. |
| Calidad inconsistente de outputs agénticos | Alto | Eval harness con golden datasets, gates de calidad, *human review* en muestras. |
| Resistencia cultural ("la IA me reemplaza") | Medio | Posicionamiento explícito como *augmentation*; KPIs de productividad por persona, no de reducción. |
| Deuda técnica en assets generados | Medio | Lint/format/coverage como gates; *refactoring agent* programado periódicamente. |
| Compliance / regulación (GDPR, sectoriales) | Alto | Data classification, residency por tenant, DPIA por flujo, mapping a estándares. |

---

## 22. Decisiones Resueltas (v0.2)

Todas las preguntas abiertas de la v0.1 fueron resueltas. Se mantienen aquí como índice rápido; el detalle de implementación está en **§24 Adendum v0.2**.

| # | Tema | Decisión | Detalle |
|---|------|----------|---------|
| 1 | Cloud target | **GCP-only en v1**, Azure como segundo cloud en Fase 5 | §24.1 |
| 2 | Workflow Engine | **Temporal** con capa visual propia (estilo n8n/Flowise) | §24.2 |
| 3 | Modelo de despliegue | **Híbrido:** Forge-Managed *(turnkey)* + Target-Project *(BYO infra del cliente)* | §24.3 |
| 4 | Consumo de modelos | **LiteLLM** como AI Gateway; **Vertex AI** + **Microsoft Foundry** como proveedores base | §24.4 |
| 5 | Mejoras autónomas | Requieren aprobación humana por defecto; autonomía solo si está **declarada explícitamente** en el spec | §24.5 |
| 6 | Cross-tenant | Capacidades **compartidas cross-tenant**; flujo end-to-end requiere **aprobación de admin del tenant** sobre el set de assets utilizables | §24.6 |
| 7 | Ownership agéntico | El agente es owner pero con **trazabilidad estricta**; despliegue en GH org de Forge **o** en GH org del cliente con GitHub App | §24.7 |
| 8 | Modelo organizacional | **Focals** por fase del SDLC + **pool de perfiles** rotables | §24.8 |

### Decisiones Resueltas en v0.3

Las 6 open questions de v0.2 quedaron resueltas. Detalle en **§25**.

| # | Tema | Decisión | Detalle |
|---|------|----------|---------|
| NQ.1 | SLA del AI Gateway | LiteLLM cubre el switching/fallback; **no se requiere tercer proveedor** | §25.1 |
| NQ.2 | Temporal Cloud vs self-hosted | **Self-hosted** (operación propia por ahora) | §25.2 |
| NQ.3 | Retención de decision logs y prompts | **30 días** en almacenamiento operacional | §25.3 |
| NQ.4 | Onboarding Modo B | **Self-service + assisted** (modelo tier) | §25.4 |
| NQ.5 | Validación cruzada de specs cross-tenant | **Focal de la fase** que impulsa el asset es owner técnico y aprobador | §25.5 |
| NQ.6 | Identidad agéntica federada | **GitHub App por tenant** (una instalación dedicada por tenant) | §25.6 |

### Nuevas Open Questions (v0.3)

Surgen tras las decisiones operativas y de stack:

1. **Contratos entre Go ↔ Python:** ¿usamos **gRPC + Protobuf** o **OpenAPI + REST** para los contratos entre el control plane (Go) y el plano agéntico (Python)? Recomendación inicial: gRPC con Protobuf como *single source of truth*, generación de stubs en ambos lenguajes.
2. **Topología HA/DR del Temporal self-hosted:** ¿multi-zona dentro de una región en v1, multi-región en v2? Definir RTO/RPO.
3. **Cold archive más allá de 30 días:** algunas industrias/regulaciones podrían exigir retención de logs de auditoría > 30 días. ¿Definimos un archivo frío en BigQuery/Cloud Storage con políticas distintas para *audit logs* vs *prompts/decision logs operacionales*?
4. **Backup y disaster recovery de Milvus:** los embeddings y colecciones de vectores son caros de regenerar. Definir estrategia de snapshot y replicación.
5. **Rotación de credenciales de las GitHub Apps por tenant:** ¿qué cadencia (90 días?) y qué proceso automático aplicamos?

---

## 23. Referencias

- **OpenSpec** — https://openspec.dev/
- **Model Context Protocol (MCP)** — https://modelcontextprotocol.io/
- **Agent Skills** — https://agentskills.io/home
- **Agent Development Kit (ADK)** — https://adk.dev/
- **Agent-to-Agent (A2A)** — https://github.com/a2aproject/A2A
- **OpenClaw** — https://openclaw.ai/
- **n8n** — https://n8n.io/
- **Flowise** — https://flowiseai.com/
- **Keycloak** — https://www.keycloak.org/
- **OpenFGA** — https://openfga.dev/
- **Backstage (CNCF)** — https://backstage.io/
- **Temporal** — https://temporal.io/
- **OpenTelemetry** — https://opentelemetry.io/
- **OWASP Top 10 for LLM Applications** — https://owasp.org/www-project-top-10-for-large-language-model-applications/

---

> **Próximos pasos sugeridos:**
> 1. Validar nombres (Forge / Alfred) con stakeholders clave.
> 2. Aprobar el Adendum v0.2 (§24) y socializarlo con focals.
> 3. Resolver las nuevas open questions (§22) en un workshop de 2 horas.
> 4. Aprobar Fase 0 y formalizar el modelo de Focals (§24.8) — identificar focals titulares y backups.
> 5. Definir 3 equipos piloto para Fases 2–3, mezclando Modo A (turnkey) y Modo B (target-project) para validar ambos flujos.

---

## 24. Adendum v0.2 — Decisiones de Producto (detalle)

Esta sección documenta en profundidad cada decisión resuelta entre v0.1 y v0.2, con su impacto en arquitectura, implementación y operación.

### 24.1 Cloud Strategy — GCP-first, Azure como segundo cloud

**Decisión:** GCP es el cloud primario en v1. Azure entra como segundo cloud en **Fase 5**, no como un genérico "multi-cloud".

**Implicancias arquitectónicas:**
- Las abstracciones de plataforma (deployment, IAM, secrets, observability) se diseñan **cloud-aware desde el día 1**, aunque solo GCP se implemente.
- Definir **interfaces** (`DeploymentTarget`, `SecretStore`, `IdentityProvider`, `ContainerRuntime`) que en Fase 5 se extienden a sus equivalentes Azure:
  - GKE → AKS
  - Cloud Run → Azure Container Apps
  - Secret Manager → Azure Key Vault
  - Vertex AI → Microsoft Foundry (ya cubierto vía LiteLLM, ver §24.4)
  - Cloud IAM → Entra ID (federado vía Keycloak)
- IaC con Terraform usando **modules abstraídos por capability**, no por servicio cloud específico.

**Trade-off explícito:** se acepta cierta sobre-ingeniería de interfaces en v1 a cambio de evitar un *big-bang refactor* en Fase 5.

---

### 24.2 Workflow Engine — Temporal con UX inspirada en Flowise

**Decisión:** Temporal-based, **no** Flowise self-hosted.

**Razonamiento:**
- Flujos agénticos productivos son **long-running** (minutos a semanas), con dependencias humanas (HITL), llamadas externas, retries con backoff exponencial y necesidad de *replay* determinístico para debugging.
- Temporal resuelve durabilidad, checkpointing, retries y versionado de workflows nativamente; Flowise/n8n no están diseñados para este perfil de carga.
- Pero la UX visual de Flowise/n8n **es valiosa** para PMs, designers y POs que quieren componer flujos sin tocar código.

**Implementación:**
- **Backend:** Temporal Cluster (decisión Cloud vs self-hosted en open question §22.NQ.2).
- **Frontend:** editor visual propio sobre Temporal Workflow API + DSL declarativo en YAML, con biblioteca de nodos pre-aprobados (LLM, MCP, Skill, HITL, Eval, Branch, Loop, Retry, Webhook).
- **Compatibilidad:** se puede importar/exportar definiciones en formato compatible con Flowise para facilitar migraciones desde POCs existentes.

---

### 24.3 Modelo de Despliegue Híbrido

**Decisión:** Forge soporta **dos modos de despliegue** para iniciativas/apps construidas con la plataforma.

#### Modo A — Forge-Managed (turnkey)

- La iniciativa se despliega en la **infraestructura del propio Forge** (proyecto GCP gestionado por la plataforma).
- Forge provee namespace en GKE / servicio en Cloud Run, dominio (`*.forge.<empresa>.com`), certificados, observability, networking, secrets.
- **Repo Git:** GitHub org de la plataforma (`github.com/<forge-org>/<solution>`).
- **Ideal para:** POCs, MVPs internos, herramientas departamentales, casos donde no se requiere aislamiento de billing o compliance específico.
- **SLA:** turnkey en minutos.

#### Modo B — Target-Project (BYO infra)

- La iniciativa se despliega en un **proyecto GCP destino** (futuramente Azure subscription) elegido por el cliente/equipo.
- Permite billing separado, políticas de seguridad propias, residency específico, integración con redes corporativas existentes.
- **Repo Git:** GitHub org especificada por el cliente; Forge se conecta vía **GitHub App** instalada por el admin del tenant con permisos de push a los repos autorizados.
- **Onboarding requerido:**
  1. Cliente pre-aprovisiona una **Service Account** de Forge en su proyecto destino, con roles mínimos definidos por Forge (publicado como Terraform module).
  2. Cliente instala la **GitHub App** de Forge en su organización Git.
  3. Forge ejecuta validación automática de permisos y conectividad antes del primer deploy.
- **Ideal para:** apps productivas, datos sensibles, equipos con políticas de cumplimiento propias.
- **SLA:** habilitación asistida en horas/días dependiendo del tenant.

#### Diseño técnico

- Una entidad **`DeploymentTarget`** en el Asset Registry parametriza Modo A/B con:
  - `mode: "managed" | "target_project"`
  - `gcp_project_id`, `gke_cluster` o `cloud_run_region` (Modo B)
  - `service_account_email` (Modo B)
  - `git_org`, `git_repo`, `github_app_installation_id` (Modo B)
  - `domain_strategy`, `network_policy`, `secret_backend`
- El SRE Agent y Alfred son **agnósticos al modo**: usan la abstracción `DeploymentTarget` para todas las acciones operativas.

---

### 24.4 AI Gateway con LiteLLM — Vertex AI + Microsoft Foundry

**Decisión:** todo consumo de LLMs en Forge ocurre exclusivamente vía **LiteLLM** (https://www.litellm.ai/) como AI Gateway. **Vertex AI** (GCP) y **Microsoft Foundry** (Azure) son los proveedores base en v1.

**Beneficios:**
- **Switching rápido** entre proveedores y modelos sin tocar código de agentes (configuración).
- **Observability central** — todas las llamadas LLM pasan por un solo punto: tokens, latency, costo, errors.
- **Fallback automático** entre proveedores ante degradación.
- **Rate limiting** y **budget caps** por tenant/proyecto/agente.
- **Guardrails uniformes** — políticas de input/output filtering centralizadas.
- **Prompt registry** integrable.
- **Cost tracking** y attribution por iniciativa.

**Implementación:**
- LiteLLM desplegado en GKE como **componente core** de la plataforma, en alta disponibilidad.
- Todos los agentes y workflows consumen `litellm-gateway.forge.svc` — nunca hablan directo a Vertex AI o Foundry.
- Configuración por tenant define modelos permitidos, budgets y políticas.
- Telemetría LiteLLM exportada a OpenTelemetry → dashboards en Langfuse/Grafana.

**Política de proveedores:**
- v1: Vertex AI (Gemini family) + Microsoft Foundry (catálogo Azure incl. modelos open-weights y propietarios).
- v2+: agregar proveedores adicionales solo si pasan revisión de Security + Data Privacy + costo.

---

### 24.5 Política de Mejoras Autónomas

**Decisión:** **aprobación humana por defecto** para todo cambio o mejora propuesta por un agente. La autonomía es **opt-in explícita**.

#### Mecánica

1. Por defecto, los agentes operan en modo **propuesta**: pueden generar PRs, sugerir refactors, proponer optimizaciones, pero **nada se mergea ni se despliega sin HITL**.
2. La autonomía se habilita solo si el OpenSpec del componente (o la configuración del proyecto) declara explícitamente:
   ```yaml
   autonomous_improvements:
     enabled: true
     allowed:
       - dependency_bumps_minor
       - lint_fixes
       - coverage_refactors
       - typo_fixes_in_docs
     forbidden:
       - public_api_changes
       - schema_migrations
       - infra_changes
   ```
3. Cualquier acción autónoma **registra en su decision log** la cláusula del spec que la autorizó (con hash del spec en ese momento).
4. El admin del tenant puede revocar la autonomía global o por tipo en cualquier momento desde el Portal.

#### Auditoría
- Vista en el Portal: "Acciones autónomas últimas 24h / 7d / 30d" con filtros por agente, tipo, repo.
- Alerta si la tasa de acciones autónomas crece anómalamente.
- Reporte semanal automático para focals de Security y Architecture.

---

### 24.6 Modelo Cross-Tenant — Capacidades Compartidas, Orquestación Aprobada

**Decisión:** dos planos de gobernanza distintos.

#### Plano de Capacidades — Cross-tenant (compartido)

- El **AI Asset Registry es global** y compartido por toda la organización.
- Cualquier tenant puede:
  - Descubrir MCPs, Skills y Agents publicados.
  - Consultar su documentación, ejemplos y *playgrounds*.
  - Proponer mejoras (vía PR al asset original).
  - Contribuir nuevos assets (siguiendo el lifecycle `proposed → in-review → approved`).
- Modelo open-source-like internamente: review por owners + Security + Architecture focals.

#### Plano de Orquestación End-to-End — Per-tenant (aprobación explícita)

- Para que un asset pueda usarse en un **flujo orquestado intent→infra de un tenant**, un **admin del tenant debe aprobarlo explícitamente** y agregarlo al *approved set* del tenant.
- Los agentes orquestadores (Alfred + sub-agentes) solo pueden invocar assets dentro del approved set durante un flujo end-to-end automatizado.
- Esto evita que un MCP experimental publicado por otro equipo se cuele accidentalmente en un pipeline productivo.

#### Implementación

- Modelo OpenFGA con relaciones:
  ```
  asset → published_in → registry (global)
  asset → approved_for → tenant (per-tenant)
  workflow → can_use → asset (solo si approved_for tenant del workflow)
  ```
- El Portal muestra al admin del tenant una vista "Assets disponibles vs aprobados" con un botón de approve/revoke.
- Auditoría: cada cambio en el approved set se registra con autor, timestamp y justificación opcional.

---

### 24.7 Ownership Agéntico y Prácticas de Colaboración

**Decisión:** los agentes son **owners de los artefactos que producen**, pero deben seguir prácticas de colaboración trazables y auditables. La identidad y los permisos del agente difieren entre Modo A y Modo B (ver §24.3).

#### Identidad agéntica

- Cada agente productivo tiene una **identidad distinguible** en GitHub/GitLab, ej:
  - `forge-po-agent[bot]`
  - `forge-dev-agent[bot]`
  - `forge-qa-agent[bot]`
  - `forge-sre-agent[bot]`
- **Commits firmados** con GPG/Sigstore.
- **Email de identidad:** `<agent-id>@bots.forge.<empresa>.com`.

#### Trazabilidad obligatoria en cada commit/PR

```
Commit message:
  feat(orders): add idempotency key to checkout endpoint

  Implements OpenSpec OS-1234 (epic: checkout-resilience).

  Forge-Agent: forge-dev-agent[bot]@v2.3.1
  Forge-Workflow: wf_01HXY...
  Forge-Initiator: jsmith@empresa.com
  Forge-Spec: OS-1234#section-3.2
  Signed-off-by: forge-dev-agent[bot] <forge-dev-agent@bots.forge.empresa.com>
```

#### Code review

- Por defecto, **todo PR de agente requiere review humano** (CODEOWNERS apunta al focal de la fase + tech lead del producto).
- Excepción: solo si `autonomous_improvements.enabled = true` para ese tipo de cambio (§24.5).

#### Targets de despliegue Git

| Modo | Repo Git | Permisos del agente |
|------|----------|---------------------|
| **Modo A — Forge-Managed** | GitHub org de la plataforma Forge (`github.com/<forge-org>/<solution>`) | Permisos directos vía la GitHub App de la plataforma |
| **Modo B — Target-Project** | Org/repo especificado por el cliente | GitHub App de Forge **instalada por el admin del tenant** con permisos de push limitados a repos autorizados |

#### Buenas prácticas obligatorias

- **Branch naming:** `forge/<workflow-id>/<feature-slug>` (ej: `forge/wf_01HXY/idempotency-key`).
- **PR description estructurada** (autogenerada): qué cambia, por qué, link al OpenSpec, link al workflow execution, checklist de quality gates.
- **Conventional commits** (`feat:`, `fix:`, `chore:`, etc.).
- **Sin force-push** a branches protegidas (`main`, `release/*`).
- **Linkage automático** a Jira issue (vía smart commits).
- **Squash merge obligatorio** para mantener historial limpio.
- **Branch protection rules** desplegadas automáticamente en cada repo Modo A; recomendadas en Modo B (Forge valida y notifica al admin si no están).

---

### 24.8 Modelo Organizacional — Focals + Pool de Perfiles

**Decisión:** la operación de Forge se estructura con un **modelo de Focals por fase del SDLC** + un **pool de perfiles rotables** que distribuyen la carga.

#### Focals (titular + 1-2 backups por fase)

| Fase del SDLC | Focal responsable de... |
|---------------|-------------------------|
| **Discovery / Product** | Calidad del PO Agent, conversión intent → backlog, métricas de adopción del flujo de descubrimiento |
| **Design (UI/UX)** | Calidad del Design Agent, design tokens, integración con Figma, *design-to-code* |
| **Architecture** | Calidad del Architect Agent, ADRs, threat modeling, fitness functions |
| **Development** | Calidad del Dev Agent, scaffolds, MCPs de Git, conventions de código |
| **QA / Testing** | Calidad del QA Agent, eval harness, golden datasets, test data synthesis |
| **Security** | SAST/DAST/SCA, OWASP/LLM Top 10, red teaming, gestión de findings |
| **DevOps / SRE** | Pipelines, IaC, observability stack, self-healing, runbooks |
| **Data / IA Platform** | LiteLLM gateway, prompt registry, model routing, evals, costos AI |

**Responsabilidades del focal:**
- Owner de los KPIs y SLAs de su fase.
- Curador de los assets (MCPs/Skills/Agents) de su dominio en el Registry.
- Punto de escalación para tenants con problemas en esa fase.
- Representante en el Governance Council quincenal.

#### Pool de Perfiles (rotables, distribuidos por demanda)

- **Engineering Platform** — construcción y mantenimiento de Forge mismo (Portal, Workflow Engine, IAM, etc.).
- **AI/ML Engineers** — desarrollo de MCPs, Skills, Agents; tuning de prompts; evals.
- **Security Engineers** — guardrails, red teaming, compliance.
- **SREs** — operación, observability, self-healing, incident response.
- **Solution Architects** — soporte a tenants en arquitectura de soluciones complejas.
- **Tech Writers / DevX** — documentación, onboarding, *developer experience*.

**Distribución:** los perfiles del pool no están fijos a una fase; se asignan a iniciativas según demanda y skill match. Esto permite escalar fases con picos de carga sin reorganizar.

#### Governance Council

- **Composición:** todos los focals + Product Owner de Forge + Security Lead + Architecture Lead.
- **Cadencia:** quincenal (90 minutos).
- **Agenda fija:**
  - KPIs por fase.
  - Nuevos assets propuestos al Registry.
  - Incidentes y postmortems relevantes.
  - Cambios en políticas (autonomía, cross-tenant, deployment targets).
  - Roadmap update.

#### Sizing inicial sugerido (Fase 0–1)

| Rol | Personas |
|-----|----------|
| Focals (8 fases × 1 titular) | 8 (pueden ser part-time inicialmente) |
| Engineering Platform | 4–6 |
| AI/ML Engineers | 3–4 |
| Security | 2 |
| SRE | 2 |
| DevX / Tech Writer | 1 |
| **Product Owner de Forge** | 1 |
| **Total fundacional** | **~20–25 personas**, con escalamiento progresivo |

---

## 25. Adendum v0.3 — Decisiones Operativas y Stack Tecnológico

Esta sección documenta las decisiones operativas que cierran las open questions de v0.2 y formaliza el stack tecnológico de implementación.

### 25.1 SLA del AI Gateway — LiteLLM como única abstracción

**Decisión:** **no** se requiere un tercer proveedor de respaldo. **LiteLLM** ya implementa el switching y fallback entre Vertex AI y Microsoft Foundry, lo que cubre el escenario de degradación.

**Implicancias:**
- Configurar en LiteLLM las **rutas de fallback** (`fallback_models`, `cooldown_time`, `num_retries`) entre Vertex AI y Foundry por *capability tier* (ej: razonamiento avanzado, generación de código, embeddings).
- Definir **policies por workflow crítico** que toleran degradación (ej: usar modelo más pequeño si el grande no responde) vs flujos que prefieren fallar rápido.
- Si en el futuro se observa que ambos proveedores se degradan juntos con frecuencia inaceptable, se reevalúa la decisión.

### 25.2 Temporal — Self-hosted

**Decisión:** Temporal se opera **self-hosted** sobre GKE.

**Topología v1 sugerida:**
- Cluster Temporal en GKE Autopilot, **multi-zona** dentro de una región GCP.
- Persistencia: **PostgreSQL** (Cloud SQL HA) como datastore.
- Búsqueda: Elasticsearch o OpenSearch para *visibility*.
- Workers (Go y Python) desplegados como Deployments separados por *task queue*.

**Operación:**
- Backups automáticos del datastore con RPO ≤ 1h.
- Métricas Temporal exportadas a Prometheus → dashboards en Grafana.
- *Workflow versioning* aplicado disciplinadamente para evitar bricks de runs en vuelo.

> El upgrade a Temporal Cloud queda como decisión revisable a 12 meses con base en TCO real, esfuerzo operativo y crecimiento de carga.

### 25.3 Retención — 30 días operacional

**Decisión:** los **decision logs de agentes** y los **prompts ejecutados** se retienen **30 días** en almacenamiento operacional.

**Diseño:**
- **Hot store (30d):** PostgreSQL + object store en GCS para payloads grandes; consultas rápidas desde el Portal.
- **Audit logs (separados):** los logs de auditoría de seguridad y compliance se mantienen según política regulatoria del tenant (mapeo en su contrato), pueden ir a un cold archive en BigQuery / GCS Coldline → resolver en open question §22.NQ.3.
- **Postmortems e incidentes:** retención independiente, sin caducidad automática (son artefactos de aprendizaje organizacional).

**Rationale:** 30 días cubre el ciclo típico de debugging, eval y rollback. Compliance de mayor plazo se resuelve con archive, no con hot storage.

### 25.4 Onboarding Modo B — Self-Service + Assisted (modelo tier)

**Decisión:** modelo **híbrido tier**.

| Tier | Cuándo aplica | SLA |
|------|---------------|-----|
| **Self-service** | Caso estándar: proyecto GCP sin políticas custom, red estándar, IAM por defecto | Habilitación en **< 1 hora** vía wizard del Portal + Terraform module ejecutado por el tenant |
| **Assisted** | Casos complejos: VPCs custom, perimeter security (VPC-SC), políticas de Org Policy específicas, integración con redes corporativas, requirimientos de compliance específicos | **2–5 días hábiles**, con sesión de scoping + acompañamiento del Pool de Solution Architects |

**Componentes del onboarding self-service:**
1. **Wizard en el Portal** — recolecta `gcp_project_id`, regiones permitidas, dominio, contacto admin.
2. **Terraform module publicado** que el tenant ejecuta en su proyecto destino — crea SA, otorga roles mínimos, configura networking básico.
3. **GitHub App por tenant** (ver §25.6) — instalada por el admin del tenant en su org Git.
4. **Validador automático** — Forge ejecuta una serie de health checks (permisos, conectividad, DNS) y reporta al Portal.
5. **Smoke deploy** — un servicio "hello-forge" se despliega automáticamente como prueba.

**Escalación:** el wizard detecta condiciones complejas (red privada, políticas restrictivas) y deriva automáticamente al tier asistido.

### 25.5 Ownership de Specs Cross-Tenant — Focal de la Fase

**Decisión:** la **calidad y aprobación de specs OpenSpec** de assets compartidos cross-tenant (MCPs, Skills, Agents) recae en el **focal de la fase** que impulsa ese asset.

**Modelo de responsabilidad:**

| Rol del Focal | Responsabilidades sobre assets de su fase |
|---------------|-------------------------------------------|
| **Approve / Reject de MRs** | Cualquier cambio al asset o a su spec requiere aprobación del focal de la fase correspondiente |
| **Visión de capacidades agénticas** | Define qué MCPs/Skills/Agents debería tener su fase, prioridades de roadmap |
| **Owner técnico** | Garantiza calidad del código, documentación, evals, runbooks |
| **Curador del catálogo** | Decide qué assets entran al "core kit" recomendado para nuevos tenants |
| **Spec quality gate** | Valida que el OpenSpec esté completo, testeable y trazable antes de aprobar |

**Esto extiende el modelo de Focals (§24.8):** los focals no son solo dueños de la fase operativa, sino también **curators del catálogo de capacidades agénticas** de su dominio.

**Workflow de un asset compartido:**
```
[Cualquier contribuidor] propone → MR al repo del asset
     │
     ▼
[CI] ejecuta tests + evals + lint del spec
     │
     ▼
[Focal de la fase] revisa, comenta, aprueba/rechaza
     │
     ▼ (si aprueba)
[Asset Registry] versión actualizada, disponible cross-tenant
     │
     ▼
[Admins de cada tenant] deciden si lo agregan a su approved set (§24.6)
```

### 25.6 Identidad Agéntica Federada — GitHub App por Tenant

**Decisión:** cada tenant tiene su **propia GitHub App de Forge instalada**, no se usa una sola App con múltiples instalaciones compartidas.

**Pros del enfoque elegido:**
- **Aislamiento de blast radius:** un compromiso de credenciales afecta solo a un tenant.
- **Rotación independiente** de secretos por tenant.
- **Branding y trazabilidad clara:** cada tenant ve "Forge Agent for `<Tenant Name>`" en sus PRs.
- **Permisos explícitamente otorgados** por el admin de cada tenant, sin pre-existir.
- **Audit logs limpiamente separados** por tenant.

**Contras aceptados:**
- Mayor overhead operativo: una App por tenant implica más identidades a gestionar, más rotaciones, más metadatos.
- Forge debe operar un **servicio de gestión de GitHub Apps** que orqueste creación, rotación y revocación.

**Diseño del servicio de gestión:**
- Cada GitHub App registrada en Forge tiene metadata: `tenant_id`, `app_id`, `installation_id`, `private_key_secret_ref` (Secret Manager), `permissions_scope`, `created_at`, `last_rotated_at`.
- **Rotación automática** programada (cadencia a definir en open question §22.NQ.3 — recomendación inicial 90 días).
- **Revocación automática** si el tenant es offboarded o si un audit detecta uso anómalo.
- **Acceso a la private key** solo desde los workers que ejecutan acciones agénticas, vía short-lived tokens.

**Onboarding del GitHub App por tenant** (parte del flujo §25.4):
1. Forge genera un manifest de la App con permisos mínimos.
2. Admin del tenant hace clic en el wizard → flujo OAuth de GitHub para crear e instalar la App en su org.
3. GitHub callback registra `installation_id` en Forge.
4. Forge valida permisos y guarda credenciales en Secret Manager.

### 25.7 Stack Tecnológico — Polyglot por Dominio

**Decisión:** Forge se construye con un stack **polyglot** donde cada lenguaje resuelve lo que mejor sabe hacer.

#### Distribución de responsabilidades

| Plano | Lenguaje / Framework | Justificación | Ejemplos de servicios |
|-------|----------------------|---------------|------------------------|
| **Control Plane / Platform Core** | **Go** | Performance bajo carga, baja huella memory, fast startup, ecosistema CNCF (k8s, Temporal), ideal para gateways y proxies | API Gateway interno, Asset Registry, Auth Proxy, Workflow Engine glue, K8s operators, Webhook receivers, Event consumers |
| **Agentic Plane** | **Python + FastAPI** | Ecosistema AI/ML dominante (ADK, LangChain, LlamaIndex, Pydantic AI, Langfuse SDK), iteración rápida, librería de modelos | Agents (PO, Architect, Dev, QA, SRE, Alfred), MCP servers internos, Eval harness, RAG pipelines, Prompt management |
| **Frontend** | **Next.js 14+ (App Router) + React** + shadcn/ui + Tailwind | DX excelente, SSR/SSG, ecosistema rico para UI compleja | Developer Portal, Admin console, Workflow editor visual |
| **Workflow Engine** | **Temporal** (workers en Go y Python) | Durabilidad, checkpointing, retries (ver §25.2) | Orquestación de flujos largos |

#### Datos y mensajería (compartidos entre planos)

| Capa | Tecnología | Notas |
|------|------------|-------|
| **Relacional** | **PostgreSQL** (Cloud SQL HA) | Schema-per-bounded-context; migraciones con Atlas o golang-migrate |
| **Cache** | **Redis** (Memorystore cluster) | Sessions, rate limits, hot data agéntico, cola de tareas livianas |
| **Vector DB** | **Milvus** | Self-hosted en GKE; colecciones por tenant; backups a GCS |
| **Event backbone** | **Apache Kafka** (Strimzi en GKE) | Topics: `agent.actions`, `workflow.events`, `telemetry`, `audit`; particionado por tenant |
| **Object storage** | **Cloud Storage (GCS)** | Artefactos, logs frizos, backups, payloads grandes |
| **Search** | OpenSearch o Elasticsearch | Para Temporal visibility + búsqueda en logs/specs |
| **Knowledge graph** | Neo4j | Relaciones entre servicios, owners, OpenSpecs, deployments |

#### Contratos entre servicios

> **Recomendación (open question §22.NQ.1):** **gRPC + Protobuf** como contrato canónico entre Go y Python, con generación automática de stubs en ambos lenguajes. Para APIs externas hacia el Portal y consumidores third-party, se expone un **gateway REST/OpenAPI** generado desde los mismos Protos. Una sola fuente de verdad de los schemas.

#### Empaquetado y despliegue

- **Contenedores** OCI con multi-stage builds.
- **Imágenes base distroless** para servicios Go; `python:3.12-slim` para FastAPI.
- **Helm charts** versionados por servicio.
- **Argo CD** para GitOps en Modo A; **Cloud Build + Argo CD remoto** para Modo B.

#### Observabilidad

- **OpenTelemetry SDKs** (Go y Python) instrumentando todo desde día 1.
- **Métricas estándar obligatorias** por servicio: RED (Rate, Errors, Duration) + USE (Utilization, Saturation, Errors).
- **AI-specific metrics** capturadas vía LiteLLM y exportadas a Langfuse.

#### Convenciones de proyecto

- **Monorepo o multi-repo:** decisión a tomar (recomendación inicial: monorepo con Bazel o Nx para coherencia, o multi-repo con templates si la organización ya tiene cultura multi-repo).
- **Lint/format obligatorio en CI:**
  - Go: `golangci-lint`, `gofmt`.
  - Python: `ruff`, `black`, `mypy --strict`.
  - Frontend: `eslint`, `prettier`, `tsc --strict`.
- **Coverage gates:** ≥ 80 % en código nuevo.
- **Conventional commits** + **squash merge** + **branch protection**.