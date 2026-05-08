## Context

Forge se especifica end-to-end en el change meta `bootstrap-forge-platform`. Esta fase aterriza la **base mínima** sobre la cual se construirán las capacidades agénticas, de orquestación, despliegue y operación. El estado actual del repo es prácticamente vacío (solo `openspec/`, `.opencode/`, `.github/`, `.codex/`, `.claude/`); por lo tanto, este change incluye **scaffolding de monorepo** además del bootstrap de plataforma.

Restricciones heredadas:
- Nube base **GCP**; runtimes soportados GKE/Cloud Run/Minikube (apps en Fase 3, infra de plataforma en GKE/Compose en esta fase).
- **Todo acceso a LLM pasa por LiteLLM** (decisión D6 del meta).
- **Tenancy aislada** y **least privilege** desde el día uno.
- **Audit trail inmutable** y **CloudEvents** para todos los eventos.

Stakeholders directos en esta fase: SDLC Team (CoE), Plataforma, Seguridad/IAM, SRE y un equipo piloto que validará el criterio de salida.

## Goals / Non-Goals

**Goals:**
- Establecer monorepo con estructura clara por plano lógico y CI básica funcionando.
- Stack base operable en local (Compose) y en cluster (GKE/Minikube) con Helm.
- Tenancy + IAM + AuthZ + audit + eventos funcionando end-to-end con un Workspace de prueba.
- LiteLLM como **único** entrypoint a LLMs, con al menos un proveedor aprobado.
- Registry mínimo capaz de aceptar metadata, suficiente para Fase 1.
- Observabilidad correlacionable por `correlation_id` en cada request.
- Conexión a GitHub funcionando vía GitHub App.

**Non-Goals:**
- Alfred operativo (loop razonamiento/acción, RAG, decision log) → Fase 1.
- MCPs/Skills/Prompts/Workflows productivos → Fases 1 y 5.
- Trust levels enforcement avanzado, lifecycle completo del Registry y eval harness → Fase 1 y 5.
- Onboarding de apps, scaffolding por templates, pipelines de imagen + SBOM/firma → Fase 2.
- Runtimes de despliegue de apps de usuarios → Fase 3.
- Capabilities por fase del SDLC → Fase 4.
- Editor visual de workflows → Fase 5.
- Self-healing → Fase 6.

## Decisions

### D0.1 — Monorepo
**Decisión**: Monorepo con carpetas por plano lógico (`services/control-plane`, `services/alfred`, `services/registry`, `portal`, `infra`, `deploy`, `contracts`, `docs`).
**Alternativas**: multi-repo desde el inicio.
**Rationale**: Ergonomía para refactors cross-service en una etapa temprana donde los contratos están en flujo; se puede dividir más adelante si es necesario.

### D0.2 — Compose para dev local + Helm para cluster
**Decisión**: `deploy/compose/` con todos los servicios para dev local; `infra/helm/` con charts para cluster (GKE/Minikube). Terraform para infra cloud.
**Alternativas**: solo Helm; dev remoto vía skaffold.
**Rationale**: Local first acelera onboarding del SDLC Team y reduce dependencia de infra cloud durante el bootstrap.

### D0.3 — Modelo OpenFGA
**Decisión**: Tipos `tenant`, `business_unit`, `workspace`, `asset`, `repo`, `environment`, `deployment` con relaciones `parent`, `owner`, `member`, `viewer`. Sin permisos globales por defecto.
**Alternativas**: roles RBAC tradicionales.
**Rationale**: ReBAC modela naturalmente Tenant→BU→Workspace y permite delegated permissions (Fase 1) sin reescribir el modelo.

### D0.4 — Bloqueo de bypass de LiteLLM
**Decisión**: Egress restringido por NetworkPolicy a los endpoints de proveedores LLM solo desde el namespace de LiteLLM; el resto de pods no puede resolver/conectar a esos hosts.
**Alternativas**: solo policy a nivel de aplicación.
**Rationale**: Defense-in-depth: incluso código malicioso o errado dentro de un servicio no puede bypassear LiteLLM si el egress no le permite alcanzar al proveedor.

### D0.5 — Audit append-only con tamper-evidence
**Decisión**: Tabla `audit_event` con trigger que rechaza UPDATE/DELETE; cada evento incluye `prev_hash` (hash del evento anterior por `tenant_id`) para tamper-evidence ligero. Publicación replicada a Kafka topic `audit.events.v1`.
**Alternativas**: WORM externo; solo Kafka.
**Rationale**: Trigger + hash chain dan tamper-evidence suficiente en Fase 0 sin dependencias externas; Kafka soporta replay y fan-out a observabilidad/SIEM.

### D0.6 — CloudEvents v1.0 con extensiones Forge
**Decisión**: Contratos en `contracts/events/` versionados (`v1`). Extensiones obligatorias: `forgetenantid`, `forgeworkspaceid` (opcional), `forgeactor`, `forgecorrelationid`.
**Alternativas**: schema propio.
**Rationale**: Estándar abierto, herramientas, portabilidad.

### D0.7 — `correlation_id` end-to-end
**Decisión**: Middleware en Control Plane y Portal que genera o propaga `correlation_id` (UUID v7), lo inyecta en logs estructurados, atributos OTel y eventos CloudEvents.
**Alternativas**: solo trace_id de OTel.
**Rationale**: `correlation_id` sobrevive cruces async (Kafka) y agrupa una intención end-to-end (intent → audit → eventos); trace_id queda para spans y se relaciona como atributo.

### D0.8 — Registry mínimo en Fase 0
**Decisión**: Registry expone CRUD básico de asset (id, name, type, version, owner_team, description, inputs_schema, outputs_schema, lifecycle_state=`proposed`). Lifecycle completo (transitions, in_review, approved con eval thresholds) y trust levels se aterrizan en Fase 1.
**Alternativas**: full lifecycle ya en Fase 0.
**Rationale**: Bootstrap requiere desbloquear publicación; el lifecycle completo depende de Alfred + eval harness, que no existen aún.

### D0.9 — Bloqueo de owners
**Decisión**: Workspace requiere ≥1 owner; eliminar el último owner sin transferencia es rechazado.
**Alternativas**: permitir Workspace huérfano.
**Rationale**: Evita pérdida de accountability.

## Risks / Trade-offs

| Riesgo | Mitigación |
|---|---|
| Configuración compleja de OpenFGA al inicio | Modelo simple en Fase 0; iterar en Fase 1 con delegated permissions; tests de policy desde el bootstrap. |
| Bypass de LiteLLM | NetworkPolicy + egress firewall + auditoría a nivel app; alertas si un servicio intenta resolver hostnames de proveedores fuera de LiteLLM. |
| Costos de operar el stack completo en bootstrap | Compose para dev local; cluster mínimo en Minikube/GKE Autopilot pequeño; Milvus tamaño S; budgets en LiteLLM con cuota baja. |
| Audit no escalable | Particionamiento por mes y `tenant_id`; archivado a object storage frío con retención según política. |
| Keycloak self-hosted complejo | Usar imagen oficial con configuración declarativa (Realm import) versionada en repo. |
| Falsos sentidos de seguridad por audit append-only sin WORM externo | Replicación a Kafka + sink a object storage versionado/inmutable como segunda línea. |
| Drift entre Compose y Helm | Helm como source of truth; Compose generado/mantenido con un script de paridad y tests smoke comunes. |

## Migration Plan

Bootstrap puro; **no hay migración desde un sistema previo**.

Rollout:
1. Crear monorepo con CI básica.
2. Provisionar Compose dev y validar smoke local.
3. Provisionar cluster (Minikube/GKE de plataforma) y desplegar Helm charts.
4. Configurar Keycloak/OpenFGA con realms y modelo de autorización.
5. Desplegar Postgres/Redis/Kafka/Milvus.
6. Desplegar Control Plane + Portal + Audit Service + Registry.
7. Desplegar OTel + Prometheus + Grafana + Loki + Tempo.
8. Desplegar LiteLLM y aplicar NetworkPolicy de egress.
9. Configurar GitHub App.
10. Ejecutar criterio de salida e2e.

**Rollback**: cada componente se despliega vía Helm con valores versionados; rollback a versión previa del chart. Datos de prueba pueden purgarse sin afectar nada productivo (Fase 0 no tiene tenants productivos aún).

## Open Questions

1. ¿Kafka managed (Confluent Cloud) o self-hosted (Strimzi en GKE) para el bootstrap? Decisión final con SRE/Plataforma.
2. ¿Milvus self-hosted (Helm) o Zilliz Cloud para Fase 0? Influye en costo y operabilidad.
3. Política definitiva de retención de audit (días/meses) por clasificación — pendiente de Security/Compliance.
4. Política de naming de tópicos Kafka (`<domain>.<event>.v<n>`) — propuesta a confirmar.
5. Mapeo concreto de claims del IdP corporativo a tuples OpenFGA — depende de IdP team.
