# Tasks — Phase 3: Deployable Apps

## 1. Runtime registry y modelo

- [x] 1.1 Crear `services/runtime-registry/` (Go) con API REST y modelo de datos `runtime`.
- [x] 1.2 Definir tipos: `gke`, `cloudrun`, `minikube`; modos: `byo`, `provisioned`.
- [x] 1.3 Endpoint `POST /v1/runtimes` (registrar BYO con kubeconfig/SA cifrado vía KMS).
- [x] 1.4 Endpoint `POST /v1/runtimes/{id}/preflight` con validaciones (conectividad, RBAC mínimo, namespaces, registry pull access, Workload Identity).
- [x] 1.5 Eventos `runtime.registered.v1`, `runtime.preflight.v1`.

## 2. Connectors / Deployers

- [x] 2.1 Definir interfaz `Deployer` en `pkg/deployers/`.
- [x] 2.2 Implementar `pkg/deployers/minikube/` (kubectl wrapper).
- [x] 2.3 Implementar `pkg/deployers/gke/` (kubectl + Helm + Workload Identity).
- [x] 2.4 Implementar `pkg/deployers/cloudrun/` (gcloud SDK + revision traffic split).
- [x] 2.5 Tests de integración por connector (kind cluster, GKE de pruebas, Cloud Run sandbox).

## 3. Deploy orchestrator

- [x] 3.1 Crear `services/deploy-orchestrator/` (Go) con API y emisión de eventos.
- [x] 3.2 Modelo de datos: `deployment`, `deployment_event`, `deployment_policy_eval`, `image_verification_result`, `rollback_record`.
- [x] 3.3 Endpoint `POST /v1/deployments` con `request_id` idempotente.
- [x] 3.4 Pipeline de stages: preflight → policy → image-verify → render → apply → verify → notify.
- [x] 3.5 Emisión de eventos por stage: `deployment.policy_evaluated.v1`, `deployment.image_verified.v1`, `deployment.applied.v1`, `deployment.verified.v1`, `deployment.failed.v1`.
- [x] 3.6 Endpoint `POST /v1/deployments/{id}/rollback` con stages: lookup-prev-revision → policy → re-apply → verify.
- [x] 3.7 Stream SSE `GET /v1/deployments/{id}/stream`.

## 4. Image verification

- [x] 4.1 Implementar `pkg/cosign/` con `Verify` y `VerifyAttestation` contra Sigstore (Fulcio/Rekor) público y privado.
- [x] 4.2 Configurar identidades OIDC esperadas por Workspace.
- [x] 4.3 Comparación `image_digest` vs Registry; falla → bloqueo.
- [x] 4.4 Override `allow-unsigned-image` con TTL ≤ 1h.
- [x] 4.5 Tests de verificación con imagen firmada y no firmada.

## 5. Provisioned by Forge (Terraform + Helm)

- [x] 5.1 Repositorio `forge-iac-modules/`: módulos Terraform (`gcp-project`, `gke-autopilot`, `cloudrun-service`, `artifact-registry-binding`, `workload-identity`).
- [x] 5.2 Backend Terraform: bucket GCS por Tenant con state locking + encryption + versioning.
- [x] 5.3 Helm charts gobernados (`forge-app-chart`) generados desde repo template.
- [x] 5.4 Endpoint `POST /v1/runtimes/provision` orquestando Terraform apply + outputs ingestion.
- [x] 5.5 Audit completo de `terraform plan/apply` con events.

## 6. Deployment policies

- [x] 6.1 Policy templates: `require-approval-prod`, `freeze-window`, `require-signed-image`, `require-canary`, `require-rollback-plan`.
- [x] 6.2 Integrar evaluación con `services/policy-engine/` (Fase 1).
- [x] 6.3 Approvals Inbox: items de tipo `deployment-approval`.
- [x] 6.4 Emisión `deployment.policy_evaluated.v1` con detalle por policy aplicada.

## 7. IaC drift detection

- [x] 7.1 Crear `services/iac-drift/` (Go) con cron scheduler.
- [x] 7.2 Job hourly: por cada IaC workspace ejecuta `terraform plan -detailed-exitcode`.
- [x] 7.3 Modelo de datos: `iac_drift_finding`.
- [x] 7.4 Eventos `iac.drift.detected.v1` consumidos por Alfred.
- [x] 7.5 `.forge-drift-ignore.yaml` schema y validación.
- [x] 7.6 Skill `propose-drift-remediation` (extiende Fase 1) que abre PR con `terraform apply` plan diff.

## 8. Rollback

- [x] 8.1 Persistir cada `revision_id` con `(asset, env, image_digest, manifest_hash)`.
- [x] 8.2 Botón "Rollback to previous" en Portal con confirmación y razón.
- [x] 8.3 Auto-rollback ante `verify` fail (configurable por env).
- [x] 8.4 Override `allow-non-reversible-rollback` para casos con migraciones DB.
- [x] 8.5 Audit y eventos `deployment.rolled_back.v1`.

## 9. Registry & asset extension

- [x] 9.1 Extender asset `application` con sub-recurso `deployment` (release history por env).
- [x] 9.2 Endpoint `GET /v1/assets/{id}/deployments?env=prod` con paginación.
- [x] 9.3 Cada deployment registra `image_digest`, `revision_id`, `verified_status`, `openspec_ids`, `pr_sha`, `runtime_id`.

## 10. Portal — UI

- [x] 10.1 Módulo "Runtimes": listar, registrar BYO, ejecutar preflight, eliminar.
- [x] 10.2 Módulo "Deployments": historial por asset/env, status en vivo, botón rollback, detalle de stages.
- [x] 10.3 Módulo "Drift": findings, severity, remediation propuesta, aplicar PR.
- [x] 10.4 Tests E2E con Playwright cubriendo deploy a minikube y a runtime BYO.

## 11. Observabilidad

- [x] 11.1 Métricas: `deploy_duration_seconds`, `deploy_success_rate`, `image_verification_failure_rate`, `drift_findings_total`, `rollback_rate`, `time_to_recover_seconds`.
- [x] 11.2 Dashboards Grafana por Workspace y global.
- [x] 11.3 SLOs iniciales: deploy success rate ≥ 95% (dev), ≥ 99% (prod); image verification 100%; drift detection p95 ≤ 1h.

## 12. Validación y sign-off

- [x] 12.1 E2E: app onboardeada en Fase 2 → deploy a minikube → deploy a GKE BYO → deploy a Cloud Run Provisioned.
- [x] 12.2 Inyectar imagen no firmada → verificar bloqueo y override flow.
- [x] 12.3 Inyectar drift artificial en Terraform → verificar detection event y remediation PR.
- [x] 12.4 Ejecutar rollback en un deploy fallido y verificar restore.
- [ ] 12.5 Sign-off Platform + Security + Workspace piloto.
- [x] 12.6 Documentación: `docs/runtimes/`, `docs/deployments/`, `docs/iac/drift.md`, `docs/operations/rollback.md`.
