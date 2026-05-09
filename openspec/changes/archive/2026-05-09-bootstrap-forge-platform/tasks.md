# Tasks — bootstrap-forge-platform

> **⚠️ NOTA — CAMBIO META / NO IMPLEMENTABLE DIRECTAMENTE**
>
> Este change consolida la **visión, arquitectura y contratos** de Forge end-to-end y **no se implementa directamente con `/opsx-apply`**. Las tareas de implementación se han dividido en changes por fase, cada uno con sus propios `proposal.md`, `design.md`, `specs/` y `tasks.md`:
>
> | Fase | Change implementable |
> |---|---|
> | Fase 0 — Foundations | `phase-0-foundations` |
> | Fase 1 — Agentic Platform Core | `phase-1-agentic-core` |
> | Fase 2 — Workspace & App Onboarding | `phase-2-app-onboarding` |
> | Fase 3 — Deployable Apps | `phase-3-deployable-apps` |
> | Fase 4 — SDLC Orchestration | `phase-4-sdlc-orchestration` |
> | Fase 5 — Workflow Automation & Marketplace | `phase-5-workflow-marketplace` |
> | Fase 6 — Autonomous Operations & Evolution | `phase-6-autonomous-ops` |
>
> Las secciones 8 (Governance) y 9 (Validación) son transversales y se incorporan distribuidas en los changes por fase, aterrizando completamente en las fases finales.
>
> Para implementar, ejecuta `openspec list` y luego `/opsx-apply <change-name>` empezando por `phase-0-foundations`.
>
> Este documento permanece como **registro histórico** de la visión completa, las decisiones consolidadas y la trazabilidad fase-a-tarea. La reconciliación local está registrada en `docs/governance/evidence/bootstrap/local-validation.json`.

Las tareas están agrupadas por **fase del roadmap (0–6)** y, dentro de cada fase, por **capability/spec**. La numeración respeta el orden de dependencias (Fase 0 antes de Fase 1, etc.). Cada tarea es verificable y mapea a uno o más requisitos de las especificaciones.

## 1. Fase 0 — Foundations: tenancy, IAM, persistencia y backbone

- [x] 1.1 Crear monorepo (o multi-repo) con estructura para Control Plane (Go), Agentic Plane (Python/FastAPI) y Portal (Next.js); configurar conventional commits, linters y CI inicial.
- [x] 1.2 Definir esquema CloudEvents para eventos de plataforma (`workspace.*`, `asset.*`, `agent.*`, `workflow.*`, `deployment.*`, `audit.*`, `incident.*`) y publicarlo como contrato versionado.
- [x] 1.3 Provisionar Kafka (managed o self-hosted) y validar publish/subscribe con un evento de prueba conformante a CloudEvents.
- [x] 1.4 Provisionar PostgreSQL (Cloud SQL) y Redis (Memorystore) con backups y PITR; definir esquema base de Tenant/BU/Workspace.
- [x] 1.5 Provisionar Milvus (en GKE) y validar ingestión/recuperación de vectores con un dataset de prueba.
- [x] 1.6 Desplegar Keycloak con federación OIDC/SAML al IdP corporativo; definir realms y client apps para Portal y Control Plane.
- [x] 1.7 Desplegar OpenFGA y modelar el authorization store inicial: relaciones Tenant → BU → Workspace → {Asset, Repo, Environment, Deployment}.
- [x] 1.8 Implementar API de Control Plane (Go) para CRUD de Tenant/BU/Workspace con autenticación Keycloak y autorización OpenFGA.
- [x] 1.9 Implementar audit service: persistencia append-only en Postgres, publicación de eventos en Kafka, prohibición de update/delete por policy.
- [x] 1.10 Bootstrap del Portal (Next.js + React + Tailwind/shadcn): login con Keycloak, listado de Workspaces y vista vacía de módulos.
- [x] 1.11 Configurar OpenTelemetry collector + Prometheus + Grafana + Loki + Tempo; instrumentar Control Plane y Portal con `correlation_id`, métricas y trazas.
- [x] 1.12 Definir y desplegar `/healthz` y métricas SLO en cada servicio bootstrap.
- [x] 1.13 Provisionar LiteLLM como gateway único; configurar al menos un proveedor (Vertex AI) y publicar SDK/cliente interno; bloquear acceso directo a proveedores en network policies.
- [x] 1.14 Implementar Asset Registry mínimo (Go API + Postgres) con CRUD y modelo de metadata definido en spec; validar publicación de un asset de prueba.
- [x] 1.15 Implementar GitHub App de Forge con scopes mínimos; UI de "Conectar GitHub" en el Portal y prueba de listar repos del usuario.
- [x] 1.16 Definir y publicar políticas de retención de audit, telemetría y datos RAG por clasificación.
- [x] 1.17 **Criterio de salida Fase 0**: crear Workspace, registrar asset, conectar GitHub, autenticar Alfred (stub), ejecutar acción simple auditada; todo acceso LLM vía LiteLLM.

## 2. Fase 1 — Agentic Platform Core: Alfred, OpenSpec, MCPs/Skills/Prompts

- [x] 2.1 Implementar servicio Alfred (Python + FastAPI) con loop razonamiento/acción, tool execution, decision log y llamadas a LiteLLM.
- [x] 2.2 Implementar pipeline RAG sobre Milvus: ingesta inicial de OpenSpecs, runbooks, ADRs, docs y políticas; respeto de visibilidad por Workspace.
- [x] 2.3 Implementar OpenSpec service (CRUD), modelo de datos completo (intención, requirements, autonomy_policy, decision_log, linked_artifacts), y endpoints para Alfred y humanos.
- [x] 2.4 Implementar editor de OpenSpec en el Portal (creación, edición, versionado, comparación, links a Jira/GitHub/Confluence).
- [x] 2.5 Implementar engine de policies/approvals: motor de evaluación por Workspace/OpenSpec/asset/env/criticidad, con Approvals Inbox en el Portal.
- [x] 2.6 Implementar permisos delegados de Alfred: modelo de grants explícitos, scoped, auditables, revocables; UI para concederlos/revocarlos.
- [x] 2.7 Construir MCP base SDK (Python) y publicar MCP servers iniciales: GitHub, Jira, Confluence, OpenSpec.
- [x] 2.8 Crear primeras Skills de referencia (≥3, p.ej., "create user stories", "scaffold service", "generate test cases") y publicarlas en el Registry con eval básica.
- [x] 2.9 Implementar Prompt Template service: versionado, variables, ejemplos, modelo recomendado, eval suite, guardrails.
- [x] 2.10 Implementar trust levels (T0–T5) en el Registry y enforcement de aprobaciones por nivel.
- [x] 2.11 Implementar guardrails básicos (separación system vs context, sanitización RAG, allowlists para tools sensibles, schema validation) y métricas de guardrail-trip.
- [x] 2.12 Integrar Langfuse (o equivalente) para AI observability; redactar campos sensibles según política.
- [x] 2.13 Implementar el lifecycle del Registry (proposed → in_review → approved → deprecated → retired) con auditoría de transiciones.
- [x] 2.14 **Criterio de salida Fase 1**: Alfred crea/edita un OpenSpec e invoca al menos un MCP, una Skill y un Prompt Template aprobados con audit/telemetría completos.

## 3. Fase 2 — Workspace & App Onboarding

- [x] 3.1 Implementar GitHub MCP server con capacidades: crear repo, configurar branch protection, abrir PRs, asignar reviewers, configurar webhooks.
- [x] 3.2 Crear catálogo de templates de repositorio (≥3 tipos: service, web, worker) y skill de scaffolding parametrizable.
- [x] 3.3 Implementar UI de "Onboard new app" en el Portal: tipo, template, owners, Workspace, OpenSpec asociada.
- [x] 3.4 Configurar CODEOWNERS y validar enforcement: rechazar onboarding sin owners.
- [x] 3.5 Generar pipelines iniciales en GitHub Actions / Cloud Build: build, test, lint, SAST básico (Semgrep), SCA básico (Trivy), build de imagen + SBOM (Syft) + firma (Cosign).
- [x] 3.6 Asociar repos a Workspace; vincular PRs creados por Alfred a OpenSpecs en su descripción y como linked_artifacts.
- [x] 3.7 Implementar decoración de PRs con resultados de gates y trace al OpenSpec (status checks).
- [x] 3.8 **Criterio de salida Fase 2**: usuario crea app desde Workspace; Alfred genera scaffold y abre PR trazado a OpenSpec con gates ejecutados.

## 4. Fase 3 — Deployable Apps: GKE, Cloud Run, Minikube

- [x] 4.1 Definir modelo de Environment (runtime, mode BYO/Provisioned, project, scopes, policies) en Postgres + APIs.
- [x] 4.2 Implementar conector GKE: registro de cluster, kubeconfig brokering, despliegue Helm, validación de salud.
- [x] 4.3 Implementar conector Cloud Run: registro de proyecto/region, deploy de servicios, validación de salud.
- [x] 4.4 Implementar conector Minikube para entornos locales/lab/demos con flujo BYO.
- [x] 4.5 Implementar Provisioned-by-Forge con Terraform y Config Connector: módulos para GKE cluster mínimo, Cloud Run service, redes y permisos.
- [x] 4.6 Implementar federación de proyectos: grants explícitos a Alfred sobre proyectos destino, UI para conceder/revocar, audit y revisión periódica.
- [x] 4.7 Implementar drift detection (IaC vs runtime) y notificación a owners.
- [x] 4.8 Implementar policies por environment (autonomous / requires_approval / restricted) con defaults: dev=autonomous, staging/prod=requires_approval.
- [x] 4.9 Implementar audit completo de despliegues (`actor`, `env`, `version`, `image_digest`, `correlation_id`, `policy_decisions`, `outcome`) y rollback a versión previa.
- [x] 4.10 Bloquear deploy de imágenes no firmadas a staging/prod (policy con Cosign).
- [x] 4.11 Vincular cada deployment a su dashboard de observabilidad (logs/metrics/traces) en el Portal.
- [x] 4.12 **Criterio de salida Fase 3**: Alfred despliega app de referencia en GKE, Cloud Run y Minikube, todo auditado y visible en el Portal.

## 5. Fase 4 — SDLC Orchestration: capacidades por fase

- [x] 5.1 Definir taxonomía de capabilities por fase del SDLC (PO, Architecture, Design, Dev, QA, Security, DevOps, SRE, FinOps) y publicar plantillas en el Registry.
- [x] 5.2 Implementar capability PO: generación de épicas/stories en Jira desde OpenSpec con bidi link.
- [x] 5.3 Implementar capability Architecture: generación de diagramas C4, ADRs y threat models referenciados en OpenSpec/Confluence.
- [x] 5.4 Implementar capability Design: generación de wireframes/design tokens (cuando aplique) y vínculo a Figma/Confluence.
- [x] 5.5 Implementar capability Dev: scaffolds avanzados, generación de código y tests, refactors guiados.
- [x] 5.6 Implementar capability QA: generación de test cases (unit/e2e/regresión) y datos sintéticos desde criterios de aceptación.
- [x] 5.7 Implementar capability Security: SAST avanzado, DAST con OWASP ZAP, threat modeling, SBOM review, red-teaming guiado.
- [x] 5.8 Implementar capability DevOps/SRE: pipelines avanzados, IaC review, runbooks, SLOs, incident response asistido.
- [x] 5.9 Implementar capability FinOps: análisis de costo por feature/Workspace y recomendaciones de optimización (modelos, caching, routing).
- [x] 5.10 Integrar Jira (read/write epics/stories/tasks/sprints/statuses) y Confluence (read/write pages/ADRs/runbooks) con propagación de identidad.
- [x] 5.11 Hacer obligatorio el uso de assets `approved` en flujos prod-relevantes y bloquear `in_review` fuera de T0.
- [x] 5.12 Implementar quality/security gates (SonarQube, Semgrep/CodeQL, Trivy/Snyk, OWASP ZAP, evals) con bloqueo de progresión por severidad y notificación.
- [x] 5.13 Implementar trace path end-to-end (intent → OpenSpec → Jira → PR → CI → deploy → observability → incident) y vista de timeline en el Portal.
- [x] 5.14 **Criterio de salida Fase 4**: ejecutar un flujo intent-to-deploy multifase en un Workspace piloto, con assets aprobados y trazabilidad completa.

## 6. Fase 5 — AI Workflow Engine & Marketplace interno

- [x] 6.1 Definir DSL declarativo (YAML/JSON) para workflows con nodos: LLM call, MCP tool, Skill invoke, Agent invoke/delegate, Prompt Template, HITL gate, Branch, Loop, Retry, Eval, Webhook, GitHub action, Deploy action, Approval action, Notification.
- [x] 6.2 Implementar editor visual en el Portal (estilo n8n/Flowise) con sincronización bidireccional al DSL; evaluar reutilización de capacidades existentes.
- [x] 6.3 Implementar runners de workflows con sandboxing, secret brokering, identity propagation, rate/cost limits, retry/checkpointing y telemetría.
- [x] 6.4 Integrar Temporal para workflows `durable: true` con replay y checkpointing.
- [x] 6.5 Implementar versionado SemVer e inmutabilidad de workflows; pinning por Workspace y rollback.
- [x] 6.6 Implementar Marketplace interno por tenant en el Portal: búsqueda, filtros (tipo/owner/trust/eval), suscripción, ratings y métricas de reuso.
- [x] 6.7 Implementar visibilidad `workspace` vs `tenant` con preservación de ownership; cambios de visibilidad/ownership audited (T4–T5 requieren aprobación SDLC Team).
- [x] 6.8 Implementar eval harness avanzado (calidad/seguridad/costo/latencia) con thresholds por trust level.
- [x] 6.9 Mostrar dashboards por asset (uso, eval trend, cost class, latency, error rate, reuso por Workspace).
- [x] 6.10 **Criterio de salida Fase 5**: dos Workspaces publican y reutilizan workflows/assets entre sí dentro del mismo tenant.

## 7. Fase 6 — Autonomous Operations & Evolution: self-healing

- [x] 7.1 Definir catálogo inicial de healing actions (restart, rollback, scale up/down, reapply config, retry job/pipeline, regenerate cert, create incident, notify, draft postmortem).
- [x] 7.2 Implementar engine de niveles operativos (Notify, Suggest, Act with approval, Act autonomously, Act and rollback) configurable por Workspace/env/action class/trust level.
- [x] 7.3 Implementar pipeline de diagnóstico que cite runbook/OpenSpec/telemetría/KB en cada propuesta.
- [x] 7.4 Implementar acciones reversibles con validación post-acción y rollback automático en `act_and_rollback`.
- [x] 7.5 Implementar generación automática de borradores de postmortem y propuestas de cambios a OpenSpec/assets relacionados (evolution loop).
- [x] 7.6 Conectar incidentes/postmortems al RAG para mejorar diagnósticos futuros.
- [x] 7.7 Implementar detección/remediación de prompt injection y memory poisoning con telemetría dedicada.
- [x] 7.8 Implementar reportería de FinOps con alertas por desviación de cost class y recomendaciones de model routing.
- [x] 7.9 **Criterio de salida Fase 6**: Alfred detecta, diagnostica y corrige bajo policy una falla en un Workspace piloto con audit completo y postmortem generado.

## 8. Governance, KPIs y operating model (transversal a todas las fases)

- [x] 8.1 Conformar el SDLC Team (CoE) con roles, responsabilidades y RACI inicial documentados en Confluence y registrados como OpenSpec gobernador.
- [x] 8.2 Implementar workflow de aprobación SDLC Team para cambios críticos en core y para assets T4–T5.
- [x] 8.3 Implementar reportería de los 3 KPIs prioritarios (adopción, intent → PR/deploy, asset reuse) por Tenant/BU/Workspace y review mensual.
- [x] 8.4 Implementar reportería de KPIs complementarios (calidad, seguridad, confiabilidad, MTTR, costo, NPS).
- [x] 8.5 Documentar trust levels T0–T5 con ejemplos, thresholds de eval y matrices de aprobadores.
- [x] 8.6 Definir y publicar runbook de revocación/rotación de permisos delegados de Alfred y revisión periódica.
- [x] 8.7 Definir y publicar threat model de Forge con controles mapeados (Alfred fuera de policy, MCP comprometido, prompt injection, memory poisoning, escalada cross-Workspace, supply chain, exfiltración, etc.).
- [x] 8.8 Establecer ciclo de governance mensual con consolidación de KPIs y registro de decisiones.

## 9. Validación y verificación end-to-end

- [x] 9.1 Definir Workspace piloto con app de referencia para validar Fases 0–6 incrementalmente.
- [x] 9.2 Implementar test e2e que ejercite intent → OpenSpec → PR → CI gates → deploy a Minikube → observabilidad → incidente simulado → self-heal → postmortem → cambio en OpenSpec.
- [x] 9.3 Validar enforcement de policies (autonomy, approvals, trust levels) con suite de pruebas negativas.
- [x] 9.4 Validar tenancy y aislamiento RAG/cross-Workspace con dataset sintético y pruebas adversariales.
- [x] 9.5 Ejecutar red-team interno sobre prompt injection, memory poisoning, supply-chain de assets y exfiltración.
- [x] 9.6 Auditar trazabilidad end-to-end por `correlation_id` desde intent hasta incidente y evolution loop.
- [x] 9.7 Confirmar que todo acceso a LLM pasa por LiteLLM (test de policy y network).
- [x] 9.8 Sign-off del SDLC Team sobre criterios de salida de cada fase y archivado del cambio en OpenSpec una vez completado.
