# Tasks — Phase 3: Deployable Apps

## 1. Runtime registry y modelo

- [ ] 1.1 Crear `services/runtime-registry/` (Go) con API REST y modelo de datos `runtime`.
- [ ] 1.2 Definir tipos: `gke`, `cloudrun`, `minikube`; modos: `byo`, `provisioned`.
- [ ] 1.3 Endpoint `POST /v1/runtimes` (registrar BYO con kubeconfig/SA cifrado vía KMS).
- [ ] 1.4 Endpoint `POST /v1/runtimes/{id}/preflight` con validaciones (conectividad, RBAC mínimo, namespaces, registry pull access, Workload Identity).
- [ ] 1.5 Eventos `runtime.registered.v1`, `runtime.preflight.v1`.

## 2. Connectors / Deployers

- [ ] 2.1 Definir interfaz `Deployer` en `pkg/deployers/`.
- [ ] 2.2 Implementar `pkg/deployers/minikube/` (kubectl wrapper).
- [ ] 2.3 Implementar `pkg/deployers/gke/` (kubectl + Helm + Workload Identity).
- [ ] 2.4 Implementar `pkg/deployers/cloudrun/` (gcloud SDK + revision traffic split).
- [ ] 2.5 Tests de integración por connector (kind cluster, GKE de pruebas, Cloud Run sandbox).

## 3. Deploy orchestrator

- [ ] 3.1 Crear `services/deploy-orchestrator/` (Go) con API y emisión de eventos.
- [ ] 3.2 Modelo de datos: `deployment`, `deployment_event`, `deployment_policy_eval`, `image_verification_result`, `rollback_record`.
- [ ] 3.3 Endpoint `POST /v1/deployments` con `request_id` idempotente.
- [ ] 3.4 Pipeline de stages: preflight → policy → image-verify → render → apply → verify → notify.
- [ ] 3.5 Emisión de eventos por stage: `deployment.policy_evaluated.v1`, `deployment.image_verified.v1`, `deployment.applied.v1`, `deployment.verified.v1`, `deployment.failed.v1`.
- [ ] 3.6 Endpoint `POST /v1/deployments/{id}/rollback` con stages: lookup-prev-revision → policy → re-apply → verify.
- [ ] 3.7 Stream SSE `GET /v1/deployments/{id}/stream`.

## 4. Image verification

- [ ] 4.1 Implementar `pkg/cosign/` con `Verify` y `VerifyAttestation` contra Sigstore (Fulcio/Rekor) público y privado.
- [ ] 4.2 Configurar identidades OIDC esperadas por Workspace.
- [ ] 4.3 Comparación `image_digest` vs Registry; falla → bloqueo.
- [ ] 4.4 Override `allow-unsigned-image` con TTL ≤ 1h.
- [ ] 4.5 Tests de verificación con imagen firmada y no firmada.

## 5. Provisioned by Forge (Terraform + Helm)

- [ ] 5.1 Repositorio `forge-iac-modules/`: módulos Terraform (`gcp-project`, `gke-autopilot`, `cloudrun-service`, `artifact-registry-binding`, `workload-identity`).
- [ ] 5.2 Backend Terraform: bucket GCS por Tenant con state locking + encryption + versioning.
- [ ] 5.3 Helm charts gobernados (`forge-app-chart`) generados desde repo template.
- [ ] 5.4 Endpoint `POST /v1/runtimes/provision` orquestando Terraform apply + outputs ingestion.
- [ ] 5.5 Audit completo de `terraform plan/apply` con events.

## 6. Deployment policies

- [ ] 6.1 Policy templates: `require-approval-prod`, `freeze-window`, `require-signed-image`, `require-canary`, `require-rollback-plan`.
- [ ] 6.2 Integrar evaluación con `services/policy-engine/` (Fase 1).
- [ ] 6.3 Approvals Inbox: items de tipo `deployment-approval`.
- [ ] 6.4 Emisión `deployment.policy_evaluated.v1` con detalle por policy aplicada.

## 7. IaC drift detection

- [ ] 7.1 Crear `services/iac-drift/` (Go) con cron scheduler.
- [ ] 7.2 Job hourly: por cada IaC workspace ejecuta `terraform plan -detailed-exitcode`.
- [ ] 7.3 Modelo de datos: `iac_drift_finding`.
- [ ] 7.4 Eventos `iac.drift.detected.v1` consumidos por Alfred.
- [ ] 7.5 `.forge-drift-ignore.yaml` schema y validación.
- [ ] 7.6 Skill `propose-drift-remediation` (extiende Fase 1) que abre PR con `terraform apply` plan diff.

## 8. Rollback

- [ ] 8.1 Persistir cada `revision_id` con `(asset, env, image_digest, manifest_hash)`.
- [ ] 8.2 Botón "Rollback to previous" en Portal con confirmación y razón.
- [ ] 8.3 Auto-rollback ante `verify` fail (configurable por env).
- [ ] 8.4 Override `allow-non-reversible-rollback` para casos con migraciones DB.
- [ ] 8.5 Audit y eventos `deployment.rolled_back.v1`.

## 9. Registry & asset extension

- [ ] 9.1 Extender asset `application` con sub-recurso `deployment` (release history por env).
- [ ] 9.2 Endpoint `GET /v1/assets/{id}/deployments?env=prod` con paginación.
- [ ] 9.3 Cada deployment registra `image_digest`, `revision_id`, `verified_status`, `openspec_ids`, `pr_sha`, `runtime_id`.

## 10. Portal — UI

- [ ] 10.1 Módulo "Runtimes": listar, registrar BYO, ejecutar preflight, eliminar.
- [ ] 10.2 Módulo "Deployments": historial por asset/env, status en vivo, botón rollback, detalle de stages.
- [ ] 10.3 Módulo "Drift": findings, severity, remediation propuesta, aplicar PR.
- [ ] 10.4 Tests E2E con Playwright cubriendo deploy a minikube y a runtime BYO.

## 11. Observabilidad

- [ ] 11.1 Métricas: `deploy_duration_seconds`, `deploy_success_rate`, `image_verification_failure_rate`, `drift_findings_total`, `rollback_rate`, `time_to_recover_seconds`.
- [ ] 11.2 Dashboards Grafana por Workspace y global.
- [ ] 11.3 SLOs iniciales: deploy success rate ≥ 95% (dev), ≥ 99% (prod); image verification 100%; drift detection p95 ≤ 1h.

## 12. Validación y sign-off

- [ ] 12.1 E2E: app onboardeada en Fase 2 → deploy a minikube → deploy a GKE BYO → deploy a Cloud Run Provisioned.
- [ ] 12.2 Inyectar imagen no firmada → verificar bloqueo y override flow.
- [ ] 12.3 Inyectar drift artificial en Terraform → verificar detection event y remediation PR.
- [ ] 12.4 Ejecutar rollback en un deploy fallido y verificar restore.
- [ ] 12.5 Sign-off Platform + Security + Workspace piloto.
- [ ] 12.6 Documentación: `docs/runtimes/`, `docs/deployments/`, `docs/iac/drift.md`, `docs/operations/rollback.md`.
