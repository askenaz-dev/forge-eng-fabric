# Propuesta: Forge SDLC End-to-End — de Intent a Infraestructura

**Fecha:** 2026-05-13
**Estado:** Borrador para revisión
**Autor:** Guillmar Ortiz (con Claude)
**Alcance:** Meta-propuesta. Se sugiere splittear en 4 OpenSpec changes (§6).

---

## 1. Resumen ejecutivo

Esta propuesta cierra los gaps actuales del SDLC orquestado por Alfred para alcanzar cobertura **end-to-end desde captura de intent hasta infraestructura productiva**, incorpora un **catálogo de Design Systems intercambiables**, y rediseña la experiencia del **Alfred Console** para servir tanto a usuarios no-técnicos como a desarrolladores avanzados.

**Tres ejes principales:**

1. **Cerrar gaps SDLC** — UI/UX Design, Architecture, Testing, Operations e Infrastructure-as-Code.
2. **Design System Catalog** — Templates seleccionables + override por app + swap en runtime.
3. **Refactor UX del Alfred Console** — App como entidad de primera clase, vista Friendly + Advanced, detección de spec existente, comando agnóstico.

---

## 2. Estado actual

### Cobertura SDLC

| Etapa | Estado | Notas |
|---|---|---|
| Planning (intent → spec) | ✅ Implementado | Wizard + RAG |
| UI/UX Design | ❌ Fuera de scope | No existe |
| Architecture Design | 📋 Specs sin orquestar | Skills definidos, no integrados al loop |
| Development (scaffold) | ✅ Implementado | `scaffold-from-openspec` |
| Testing | 📋 Specs sin orquestar | QA skills definidos, no se invocan |
| CI/CD | ✅ Implementado | Pipeline + HITL + deploy |
| Operations | 🟡 Solo specs | Healing-engine sin ejecutor |
| Infrastructure-as-Code | ❌ No contemplado | Sin pasos IaC en workflow |

### Gaps de modelo y UX

- **"App"** existe en portal (`portal/src/app/apps/`) pero **no es entidad OpenSpec** — los specs cuelgan de workspace, no de app.
- **`/openspec`** no se parsea como slash command real — es placeholder visual; el backend solo recibe `IntentRequest`.
- **No hay detección de spec existente** — cada intent genera un `openspec_id` nuevo aunque el contenido sea duplicado.
- **Console expone tecnicismos** — workspace UUID, slashes, ejemplos crudos. No apto para usuarios no-técnicos.

---

## 3. Cobertura SDLC end-to-end

### 3.1 UI/UX Design (nuevo eje `sdlc-design`)

Skills nuevos:

- `generate-ui-blueprint@1` — Desde OpenSpec + Design System seleccionado, produce plan de pantallas y jerarquía de componentes (no pixel-perfect).
- `generate-component-stubs@1` — Componentes React tipados, sin lógica, usando tokens del DS activo.
- `validate-a11y@1` — Lint de accesibilidad sobre el blueprint.

**Fuera de scope inicial:** Figma export, mockups pixel-perfect, design exploration. Reservado para fase posterior.

### 3.2 Architecture Design (cablear lo existente)

Skills ya especificados en [openspec/specs/sdlc-architecture/spec.md](openspec/specs/sdlc-architecture/spec.md), falta integrarlos al workflow:

- `generate-api-contract@1` (OpenAPI desde OpenSpec)
- `propose-data-model@1` (entidades + relaciones)
- `lightweight-threat-model@1` (STRIDE inicial)

**Acción:** Agregar paso `architect` al workflow entre `commit-spec` y `scaffold`.

### 3.3 Testing (cablear)

Skills existentes en [openspec/specs/sdlc-qa/spec.md](openspec/specs/sdlc-qa/spec.md): `generate-test-plan`, `generate-e2e-tests`, `triage-test-failures`.

**Acciones:**
- Paso `generate-tests` post-`scaffold` → unit + e2e.
- Paso `triage` reactivo cuando `ci-build` falla → analiza logs → si confianza ≥ umbral, abre PR de fix.

### 3.4 Operations (implementar healing-engine L1-L2)

Hoy son specs sin código. Mínimo viable:

- **L1** — Detección + notificación (alerta → slack/PR con runbook auto-generado).
- **L2** — Sugerencia automática de fix (analiza logs → propone PR → requiere HITL).

Deferido a fase posterior: L3-L5 (auto-rollback, auto-scale, auto-heal sin HITL).

Skills nuevos:

- `generate-slo-definitions@1` — Desde OpenSpec, propone SLOs.
- `generate-observability-config@1` — Prometheus, OTel, dashboards Grafana.
- `propose-incident-fix@1` — Analiza incidente → PR de fix.

### 3.5 Infrastructure-as-Code (nuevo eje `sdlc-infrastructure`)

Skills nuevos:

- `generate-iac@1` — Terraform/Pulumi para el stack target (GKE, RDS, etc.), derivado de la arquitectura.
- `generate-helm-chart@1` — Chart Helm para el servicio scaffold.
- `validate-iac@1` — `terraform plan` + `tfsec` + `checkov`.
- `apply-iac@1` — Apply gated por HITL.

### 3.6 Nuevo workflow de referencia

`forge.reference.intent-to-infrastructure@1` (sucesor del `intent-to-deploy@1`):

```
intent
  → capture          (Alfred wizard)
  → spec             (commit OpenSpec)
  → architect        (API + data model + threat model)
  → design           (UI blueprint + components)         [opt-in]
  → scaffold         (código de servicio)
  → test-gen         (unit + e2e)
  → iac              (Terraform + Helm)                  [opt-in]
  → open-pr
  → ci-build + tests
  → security-gates
  → hitl-approval
  → deploy-app + deploy-infra
  → observability-setup                                  [opt-in]
  → notify
```

Pasos `design`, `iac` y `observability-setup` son **opt-in** declarables en el OpenSpec:

```yaml
targets:
  ui: true
  infra: gke
  observability: true
```

---

## 4. Catálogo de Design Systems

### 4.1 Concepto

Un Design System es un **asset registrado** (como skills/MCPs/agents) que un App declara como dependency. Tres modos:

1. **Default** — Forge Default DS (Tailwind + Geist + Radix, basado en el portal actual).
2. **Templated** — Selección de un DS del catálogo platform-provided.
3. **Custom** — Tenant aporta su propio DS (override total o parcial).

### 4.2 Modelo de datos

Nuevo `asset_type: design_system` en el registry:

```yaml
asset_type: design_system
id: ds-forge-default
version: 1.0.0
tenant_id: forge-internal
visibility: public
tokens:
  uri: registry://design-systems/ds-forge-default/tokens.json
  schema: w3c-design-tokens
components:
  primitives: [Button, Card, Chip, Badge, Sheet, Kpi, Terminal]
  uri: registry://design-systems/ds-forge-default/components.json
theme_modes: [light, dark]
typography:
  display: "Instrument Serif"
  body: "Geist Sans"
  mono: "JetBrains Mono"
framework: react+tailwind
```

### 4.3 Templates por defecto a sembrar

Propuesta inicial (4 templates, nombres tentativos — confirmar con design team):

| ID | Nombre | Identidad | Componentes | Audiencia |
|---|---|---|---|---|
| `ds-forge-default` | Forge Default | Geist + ember ramp | Radix primitives (portal actual) | Default platform |
| `ds-corporate` | Corporate | Neutral, conservador | Material-like | Apps internas empresariales |
| `ds-minimal` | Minimal | Mono palette + Inter | Headless UI | Dashboards/admin |
| `ds-marketing` | Marketing | Vibrante + Instrument Serif | Custom hero/landing | Sitios públicos |

### 4.4 Selección y swap

**En tiempo de creación de App:**

```
Wizard step "Design System":
  ● Forge Default (recomendado)
  ○ Corporate
  ○ Minimal
  ○ Marketing
  ○ Custom — Sube tu DS
```

**En runtime (swap):**

- Botón "Cambiar Design System" en la vista de App.
- Genera PR con `design.tokens.json` actualizado.
- Re-scaffold opcional de componentes marcados `ds-driven`.
- Validación de breaking changes entre versiones major.

### 4.5 Override por componente

Cada App puede definir overrides:

```yaml
design:
  template: ds-forge-default
  overrides:
    color.brand.500: "#FF5733"
    radius.default: "0.5rem"
    components.Button.variant.primary.bg: "tokens.color.brand.500"
```

---

## 5. Refactor del Alfred Console

### 5.1 App como entidad de primera clase

**Nuevo modelo jerárquico:**

```
Tenant → Workspace → App → { OpenSpecs[], DesignSystem, Skills[], MCPs[], Infrastructure }
```

**Migración:**

- Specs existentes sin App padre → bucket "Unassigned" hasta reasignación manual.
- Nueva API `POST /v1/apps` en registry.
- El wizard de creación de App ya existe en [portal/src/app/apps/new/](portal/src/app/apps/new/) — extender para capturar Design System + targets (ui/infra/observability).

### 5.2 Dos vistas: Friendly y Advanced

#### Vista Friendly (default para non-tech)

```
PLATFORM · ALFRED

¿Qué quieres construir hoy?

 ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
 │ 🚀 Nueva App     │ │ 🔧 Mejorar App   │ │ 🛠 Operar App    │
 │ Desde cero       │ │ Existente        │ │ Monitor / arreglar│
 └──────────────────┘ └──────────────────┘ └──────────────────┘

[Si "Nueva App" → conversación guiada]

Alfred: ¡Hola! Cuéntame de qué se trata. ¿Es una app para tus
        clientes, equipo interno, o público general?

> _

[Sin IDs visibles. Sin slashes. Solo lenguaje natural.]
```

**Características Friendly:**

- 3 entrypoints visuales (Nueva / Mejorar / Operar).
- Workspace y App inferidos por contexto del usuario (su default + app picker con thumbnails).
- Conversación turn-by-turn (12 turnos máx, configurables).
- Al final: "¿Quieres revisar lo que entendí?" → resumen humano-readable del spec antes de commit.

#### Vista Advanced (para devs)

Mantiene el console actual con mejoras:

- **App picker** en header (breadcrumb Workspace › App).
- Comando agnóstico `/forge` (ver §5.4).
- Acceso directo a slash commands, IDs, paste de OpenSpec YAML.
- Toggle "Modo Friendly" en el header para cambiar de vista.

### 5.3 Detección de spec existente

**Flujo al enviar intent:**

```
[Usuario describe intent]
        ↓
[Alfred extrae topics + embeddings]
        ↓
[Busca en RAG sobre specs del App actual]
        ↓
  ┌─────┴─────┐
  ▼           ▼
[Match >80%] [No match]
  ↓           ↓
"Encontré [Spec X] que parece    [Continúa flujo de
 relacionado.                     nuevo spec]
 ¿Extender o crear nuevo?"
  ↓
  ┌──────────┐
  ▼          ▼
[Extender] [Nuevo]
  ↓
[Modo edit]
  ↓
[Si Spec ya está committed Y aprobado:
 "¿Implementarlo ahora?" → salta a fase architect/scaffold]
```

**Caso "ya existe el spec":** En la vista de un spec committed, botón principal "Implementar" que dispara el workflow `intent-to-infrastructure@1` desde el paso `architect`, sin re-capturar intent.

### 5.4 Comando agnóstico

**Cambio:** `/openspec` → `/forge`.

**Justificación:** El nombre `/openspec` acopla el comando a la implementación interna (metodología OpenSpec). `/forge` es agnóstico al motor de specs y consistente con la marca de la plataforma.

**Sintaxis nueva:**

```
/forge new      title="..." intent="..." app=payments-api
/forge edit     id=spec-abc title="..."
/forge implement id=spec-abc
/forge status   id=spec-abc
/forge list     app=payments-api
```

**Migración:**

- `/openspec` queda como alias deprecado por 2 versiones.
- Mensaje legible: "El comando `/openspec` será deprecado. Usa `/forge` en su lugar."
- Documentar en changelog + runbook.

### 5.5 Otros refinamientos UX

- **Placeholder del input** en lenguaje natural por default:
  > "Cuéntame qué quieres construir o mejorar..."

  (no el slash command como hoy)
- **"Control plane offline"** debe ofrecer reintento + razón legible (no solo mostrar el placeholder).
- **Botón "Run Alfred"** → renombrar a "Continuar" (friendly) / "Enviar" (advanced).
- **Examples panel** ("Create / Edit specification") → mover a un drawer "Ejemplos" colapsable, fuera de la página principal.
- **Estado del dock** (Idle/Working/Paused) → más prominente, con CTA contextual.

---

## 6. Plan de implementación

### Sugerencia: splittear en 4 OpenSpec changes

| # | Change ID | Scope | Depende de |
|---|---|---|---|
| 1 | `app-first-class-entity` | App como entidad registry; APIs CRUD; migración de specs huérfanos | — |
| 2 | `design-system-catalog` | Asset type `design_system`; seed 4 templates; selección en App wizard; swap | #1 |
| 3 | `alfred-console-redesign` | Vista Friendly + Advanced; comando `/forge`; detección de spec existente | #1 |
| 4 | `sdlc-end-to-end` | Cablear architect/design/test/iac/observability en `intent-to-infrastructure@1`; healing L1-L2 | #1, #2 |

### Fases sugeridas

- **Fase 5 (foundation + UX):** Changes #1 + #2 + #3.
- **Fase 6 (end-to-end SDLC):** Change #4.

---

## 7. Riesgos y dependencias

- **Migración de specs huérfanos** (sin App) — necesita política y UI de reasignación.
- **Capacidad del modelo para IaC + arquitectura confiable** — empezar con plantillas guiadas (no free-form), validar con golden paths.
- **Crecimiento del catálogo de skills** — de ~5 a ~15+. Revisar budget caps de tenants y costos LLM.
- **Versionado de Design Systems** — un swap de DS major version puede romper Apps; necesita estrategia de migration + diff visual.
- **Backwards compat del comando** — `/openspec` deprecado pero soportado dos versiones para no romper scripts existentes.
- **Detección de spec existente** — dependiente de calidad del embedding y curación del RAG; umbral 80% es punto de partida, calibrar.

---

## 8. Próximos pasos

1. **Validar prioridades** — ¿Cuál de los 4 changes va primero? (Sugerido: #1 + #3 en paralelo).
2. **Diseñar mockups** de Friendly + Advanced (Figma o equivalente).
3. **Confirmar templates** de Design System con design team (los 4 sugeridos son placeholders).
4. **Convertir esta meta-propuesta** en 4 OpenSpec changes formales con tasks.md cada uno.
5. **Decidir nomenclatura final**: `/forge` vs `/spec` vs otro.
6. **Aprobar/ajustar el workflow** `intent-to-infrastructure@1` y los flags opt-in.

---

## Anexo A — Mapping de gaps a artifacts

| Gap actual | Cubierto por | Nuevo skill/spec |
|---|---|---|
| UI/UX Design ❌ | §3.1 | `sdlc-design` (3 skills) |
| Architecture 📋 | §3.2 | Cablear existentes al workflow |
| Testing 📋 | §3.3 | Cablear existentes + reactividad CI |
| Operations 🟡 | §3.4 | Implementar L1-L2 + 3 skills nuevos |
| Infrastructure ❌ | §3.5 | `sdlc-infrastructure` (4 skills) |
| App no es entidad | §5.1 | `app-first-class-entity` change |
| Design system fijo | §4 | `design-system-catalog` change |
| UX muy técnica | §5.2, §5.5 | `alfred-console-redesign` change |
| `/openspec` acoplado | §5.4 | Comando `/forge` agnóstico |
| Specs duplicados | §5.3 | Detección RAG + flujos extend/new |
