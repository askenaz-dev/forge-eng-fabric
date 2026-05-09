## Why

Con apps onboardeadas, código firmado y SBOMs publicados (Fase 2), Forge debe ahora **desplegar aplicaciones de forma gobernada** en runtimes heterogéneos: clusters propios del cliente (BYO Runtime) y entornos provisionados por Forge (Provisioned by Forge). Sin esto, todo el supply chain construido en Fase 2 termina en un artefacto que nadie puede correr de manera consistente, observable y auditable.

## What Changes

- **NUEVO**: **Runtime connectors** para destinos heterogéneos: GKE (multi-cluster), Cloud Run, Minikube/kind (dev), con interfaz común `Deployer`.
- **NUEVO**: **BYO Runtime mode**: registrar un cluster del cliente (kubeconfig + service account + scopes), validarlo (preflight checks), gobernar el deploy con policies y audit, sin requerir provisión de infra por Forge.
- **NUEVO**: **Provisioned by Forge mode**: Forge provisiona el runtime usando **Terraform** (GCP project, GKE/Cloud Run, redes, IAM) o **Config Connector** + **Helm** para charts gobernados.
- **NUEVO**: **Federated GCP projects** por Workspace o Tenant (modelo configurable), con boundaries de IAM y red.
- **NUEVO**: Servicio **deploy-orchestrator**: orquesta deploys con stages (preflight → policy → image-verify → render → apply → verify → notify), idempotente, con rollback.
- **NUEVO**: **Verification de imagen** en deploy: Cosign verify de firma + attestation antes de `apply`; sin firma → bloqueo.
- **NUEVO**: **Deployment policies**: por env (dev/stage/prod), por criticidad, por horario (freeze windows), por aprobaciones (manual gate en prod).
- **NUEVO**: **IaC drift detection**: comparación periódica del estado declarado (Terraform state / Config Connector) contra el real, con alertas y remediation propuesta por Alfred.
- **NUEVO**: **Rollback** en un click (o automático ante health check fail) con re-apply de la revisión anterior y audit completo.
- **NUEVO**: Eventos `deployment.*` (`requested`, `policy_evaluated`, `image_verified`, `applied`, `verified`, `rolled_back`, `failed`), `runtime.registered.v1`, `runtime.preflight.v1`, `iac.drift.detected.v1`.
- **NUEVO**: Registro de cada **deployment** como entidad auditable con linkage a asset, image digest, OpenSpec, PR merged, runtime, env.
- **MODIFICADO**: `app-onboarding-service` — al completar onboarding, opcionalmente provisionar runtime/env defaults (dev) si el Workspace lo solicita.
- **MODIFICADO**: `policies-and-approvals` — añadir policy templates de deployment (`require-approval-prod`, `freeze-window`, `require-signed-image`, `require-canary`).
- **MODIFICADO**: `ai-asset-registry-minimal` — extender el modelo de asset `application` con sub-recurso `deployment` que mantiene historial de releases por env.
- **Criterio de salida (E2E)**: una app onboardeada en Fase 2 se despliega a `dev` (BYO o Provisioned), pasa health check, queda registrada con audit completo, su imagen verificada con Cosign, y un drift artificial es detectado y reportado.

## Capabilities

### New Capabilities

- `runtime-connectors`: abstracción `Deployer` con implementaciones GKE, Cloud Run, Minikube/kind, y registro/preflight de runtimes.
- `byo-runtime-onboarding`: registro de clusters propios del cliente (kubeconfig + scopes), validación, audit y enforcement de boundaries.
- `forge-provisioned-runtime`: provisión gobernada de infra (Terraform / Config Connector + Helm) para Workspaces que lo soliciten.
- `deploy-orchestrator`: orquestación de deploys con stages, idempotencia, rollback y eventos completos.
- `deployment-policies`: motor de policies específicas para deploys (env, criticidad, freeze windows, aprobaciones).
- `image-verification-at-deploy`: verificación obligatoria de firma Cosign + attestation antes de aplicar al runtime.
- `iac-drift-detection`: comparación periódica IaC declarado vs real, alertas y remediation propuesta.
- `deployment-history`: historial inmutable de deployments por asset y env, con rollback en un click.

### Modified Capabilities

- `app-onboarding-service`: al onboardear, opcionalmente provisionar runtime y env defaults (dev) según preferencia del Workspace.
- `policies-and-approvals`: añadir policy templates `require-approval-prod`, `freeze-window`, `require-signed-image`, `require-canary`, `require-rollback-plan`.
- `ai-asset-registry-minimal`: extender asset `application` con sub-recurso `deployment` (release history por env).

## Impact

- **Servicios nuevos**: `services/deploy-orchestrator/` (Go), `services/runtime-registry/` (Go), `services/iac-drift/` (Go), librerías `pkg/deployers/{gke,cloudrun,minikube}/`, `pkg/iac/{terraform,configconnector}/`.
- **Datos**: tablas `runtime`, `runtime_preflight_result`, `deployment`, `deployment_event`, `deployment_policy_eval`, `image_verification_result`, `iac_drift_finding`, `rollback_record`.
- **Eventos**: `runtime.registered.v1`, `runtime.preflight.v1`, `deployment.requested.v1`, `deployment.policy_evaluated.v1`, `deployment.image_verified.v1`, `deployment.applied.v1`, `deployment.verified.v1`, `deployment.failed.v1`, `deployment.rolled_back.v1`, `iac.drift.detected.v1`.
- **Integraciones**: GCP (GKE, Cloud Run, IAM, Cloud Logging), Terraform Cloud o backend GCS, Config Connector, Helm, Cosign verify, Sigstore (Fulcio/Rekor) ya configurado en Fase 2.
- **Portal**: módulo "Runtimes" (registro/listado/preflight), módulo "Deployments" (historial, status en vivo, rollback button), módulo "Drift" (findings y remediation).
- **Riesgos**: credenciales BYO Runtime (mitigado con vault + scoped SA + rotation), drift no detectado por exclusiones (mitigado con allowlists explícitas), rollback inseguro de migraciones (mitigado con rollback plan obligatorio en `criticality≥high`).
- **Out of scope**: capabilities por fase del SDLC con Jira/Confluence (Fase 4), workflows visuales (Fase 5), self-healing autónomo (Fase 6).
