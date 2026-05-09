## Context

Fase 3 cierra el ciclo build → ship → run. Las decisiones definen cómo Forge soporta runtimes heterogéneos sin acoplarse a un solo proveedor, cómo gobierna las credenciales de BYO Runtime, qué se considera un deploy seguro (firma + policy + verificación), y cómo se maneja drift y rollback. Asume Fase 2 entregando imágenes firmadas con SBOM/SLSA y assets registrados.

## Constraints

- **No deploy sin imagen firmada**: la verificación Cosign es obligatoria; bypass requiere override gated por `security-approver`.
- **No credenciales en claro**: kubeconfig y secrets de runtimes se almacenan cifrados (KMS) y se acceden con SA scoped.
- **Idempotencia**: un mismo `deployment_request_id` aplica una sola vez; reintentos retornan el estado existente.
- **Tenancy enforcement**: un Workspace no puede deployar a un runtime de otro Workspace salvo que el runtime sea `visibility=tenant` y el Tenant lo permita.
- **Audit completo**: cada stage emite evento; el conjunto reconstruye el deploy desde la solicitud hasta la verificación.
- **Rollback obligatorio en `criticality≥high`**: el deploy debe declarar plan de rollback y lo registra antes del apply.

## Decisions

### D3.1 — Modelo de runtimes y modos

Dos modos no excluyentes por Workspace:
- **BYO Runtime**: el cliente registra un cluster (kubeconfig + service account + scopes) o un proyecto GCP con Cloud Run habilitado. Forge ejecuta deploys pero NO provisiona la infra subyacente.
- **Provisioned by Forge**: Forge provisiona el GCP project, GKE cluster (autopilot por defecto) o Cloud Run service vía Terraform; el ciclo de vida lo posee Forge.

Un Workspace puede tener ambos: BYO para prod, Provisioned para dev.

### D3.2 — Interfaz `Deployer` y connectors

```
type Deployer interface {
    Preflight(ctx, runtime) PreflightResult
    Render(ctx, manifest, params) RenderedArtifacts
    Apply(ctx, runtime, artifacts) ApplyResult
    Verify(ctx, runtime, artifacts) VerifyResult
    Rollback(ctx, runtime, prevRevision) RollbackResult
}
```

Implementaciones iniciales: `deployers/gke` (kubectl/Helm), `deployers/cloudrun` (gcloud SDK), `deployers/minikube` (kubectl). Cada connector declara capabilities (`supports_canary`, `supports_blue_green`, `supports_secrets_csi`).

### D3.3 — IaC tooling para Provisioned

- **Terraform** para infra base (GCP project, network, IAM, GKE cluster, Cloud Run service, Artifact Registry binding, Workload Identity).
- **Config Connector** para recursos GCP que necesitan ser gestionados desde Kubernetes (opcional, para Workspaces avanzados).
- **Helm** para charts gobernados de aplicación (chart oficial Forge generado del template).
- **Backend Terraform**: GCS bucket por Tenant con state locking (state encriptado + versioning).

Alternativa descartada: Pulumi — añade lenguaje adicional sin beneficio claro frente a Terraform en este scope.

### D3.4 — Federación GCP

Modelo: **un GCP project por Workspace** por env (`<workspace>-<env>`), agrupados bajo una folder por Tenant. Service Accounts scoped al project; Workload Identity binding entre GKE/Cloud Run y SA. Alternativa descartada: project compartido — viola boundary de tenancy y complica IAM.

### D3.5 — Verificación de imagen al deploy

Antes de `Apply`, el orchestrator ejecuta:
1. `cosign verify --certificate-identity-regexp=^https://github.com/<org>/.+@refs/heads/main$ --certificate-oidc-issuer=https://token.actions.githubusercontent.com <image>`
2. `cosign verify-attestation --type=slsaprovenance ...`
3. Compara `image_digest` con el registrado en el Registry; si difiere, falla.

Falla → `deployment.image_verified.v1{outcome=failed}` y bloqueo. Bypass requiere override `allow-unsigned-image` con TTL ≤ 1h.

### D3.6 — Deployment policies

Policy templates incluidos:
- `require-approval-prod`: deploys a `env=prod` requieren approval por `release-manager`.
- `freeze-window`: bloquea deploys en ventanas configurables (e.g., viernes 18:00 → lunes 08:00).
- `require-signed-image`: enforced siempre, override puntual.
- `require-canary`: en `criticality≥high` y `env=prod`, exige strategy canary o blue-green.
- `require-rollback-plan`: en `criticality≥high`, el request debe incluir `rollback_plan`.

Evaluación previa al apply; resultado emitido como `deployment.policy_evaluated.v1`.

### D3.7 — Idempotencia y revision tracking

- `deployment_request_id` (UUID provisto o generado) es la clave idempotente.
- Cada deploy genera un `revision_id` con `(asset, env, image_digest, manifest_hash, timestamp)`.
- Reintentos con mismo `request_id` retornan estado actual.
- Rollback re-apply del `revision_id` anterior; si la migration es no-reversible, requiere override `allow-non-reversible-rollback`.

### D3.8 — IaC drift detection

- Job periódico (cron 1h) ejecuta `terraform plan -detailed-exitcode` por cada workspace IaC.
- Findings de `exit=2` → `iac_drift_finding` + `iac.drift.detected.v1`.
- Alfred (Fase 1 ya operativo) recibe el evento y propone remediation (PR a `infra/` o restore via apply); en Fase 6 podrá ejecutar autónomamente bajo approvals.
- Exclusiones explícitas via `.forge-drift-ignore.yaml` versionado en repo.

### D3.9 — Health check y verify

Tras `Apply`:
- GKE: probe HTTP `/healthz` con timeout 5min, además de `kubectl rollout status` con timeout.
- Cloud Run: aguardar revision `Ready=True`, probe HTTP opcional.
- Si falla → `deployment.failed.v1` y rollback automático si está habilitado para el env.

### D3.10 — Almacén de credenciales y secrets

- Kubeconfigs y SA keys de BYO Runtime cifrados con **KMS** (Cloud KMS) por Tenant.
- Secrets de aplicación: el deployer NO inyecta secrets en claro; se referencia **Secret Manager** + **CSI driver** o `External Secrets Operator`.
- Forge nunca persiste secrets de aplicación en su DB.

## Risks / Trade-offs

- **BYO Runtime credenciales**: mitigado con KMS, scoped SA, rotation y audit por uso. La pérdida de credencial se trata como incidente de seguridad.
- **Drift de Terraform por cambios manuales**: mitigado con detection job + alertas + remediation propuesta; aceptamos que Phase 3 detecta y reporta, autónomo viene en Fase 6.
- **Verificación lenta de Cosign en Rekor privado**: mitigado con caching de attestations recientes (TTL 24h).
- **Rollback de migraciones DB**: explícitamente requiere plan; rollback automático no toca DB salvo declaración explícita en `rollback_plan`.
- **Cloud Run vs GKE feature parity**: aceptado; Cloud Run no soporta strategy canary nativa → Forge implementa traffic-splitting via revisions.

## Migration Plan

1. Implementar `runtime-registry` y connector `minikube` (dev local).
2. Implementar `deploy-orchestrator` con stages mínimos (preflight → image-verify → apply → verify) sobre minikube.
3. Implementar connector `gke` y onboardear un cluster BYO de pruebas; registrar runtime, preflight, deploy de un asset Fase 2.
4. Implementar connector `cloudrun` y deploy de servicio FastAPI Fase 2.
5. Implementar Provisioned mode con Terraform en un GCP project sandbox; provisionar GKE autopilot + Artifact Registry.
6. Implementar `iac-drift` y job de detection; introducir drift artificial y validar evento.
7. Implementar UI Portal de Runtimes / Deployments / Drift.
8. Habilitar `policies-and-approvals` extensiones (`require-approval-prod`, `freeze-window`, `require-signed-image`).
9. Sign-off Platform + Security + un Workspace piloto que despliega BYO + Provisioned.

## Open Questions

- ¿Soportar EKS/AKS en esta fase? — **Decisión inicial**: no; abstracción `Deployer` permite añadir luego sin refactor.
- ¿Service mesh (Istio/Linkerd) gestionado por Forge? — **Decisión inicial**: fuera de scope; cliente lo trae en BYO o lo solicita explícitamente.
- ¿Multi-region por defecto? — **Decisión inicial**: single-region en Provisioned; multi-region disponible vía parámetros explícitos por Workspace.
- ¿Promoción dev→stage→prod automatizada? — **Decisión inicial**: pipelines de promotion son responsabilidad del Workspace; Forge expone API y events para que workflows (Fase 5) la orquesten.
