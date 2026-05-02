# Tasks — Phase 2: App Onboarding

## 1. Repo templates y actions reutilizables

- [ ] 1.1 Crear repositorio meta `forge-templates/` con estructura por template y SemVer tags.
- [ ] 1.2 Implementar `template.yaml` schema (parámetros, validaciones, hooks, capabilities).
- [ ] 1.3 Publicar templates v1.0.0: `go-microservice`, `python-fastapi-microservice`, `nextjs-frontend`, `go-library`, `iac-terraform-module`.
- [ ] 1.4 Crear repositorio meta `forge-actions/` con composite actions: `forge/lint`, `forge/test-with-coverage`, `forge/sast`, `forge/sca`, `forge/sbom`, `forge/container-scan`, `forge/cosign-sign-attest`, `forge/publish-image`, `forge/openspec-link`.
- [ ] 1.5 Tag y firmar las versiones iniciales de templates y actions; registrar como assets en el Registry con `trust_level=T3`.

## 2. GitHub App write-mode y MCP

- [ ] 2.1 Ampliar permisos del GitHub App "Forge" (`contents:write`, `pull_requests:write`, `administration:write`, `actions:write`, `checks:write`).
- [ ] 2.2 Implementar emisión de installation tokens short-lived (≤10 min) scoped al repo objetivo.
- [ ] 2.3 Extender `services/mcp/github/` con tools: `create_repo`, `create_branch`, `create_pr`, `set_branch_protection`, `set_codeowners`, `add_pr_template`, `set_required_checks`.
- [ ] 2.4 Añadir guardrails específicos para mutaciones (allowlist de orgs, deny de mutaciones fuera del Workspace, schema validation).
- [ ] 2.5 Tests de integración del MCP write contra org de pruebas en GitHub.

## 3. Servicio de scaffolding

- [ ] 3.1 Crear `services/scaffolder/` (Go) con motor `text/template + go-getter`.
- [ ] 3.2 Implementar parsing de `template.yaml` y validación de parámetros.
- [ ] 3.3 Implementar hooks pre/post (ejecución sandboxed).
- [ ] 3.4 Implementar render con datos del Workspace (owners, criticality, data_classification).
- [ ] 3.5 Tests unitarios y golden-file por template.

## 4. Servicio app-onboarding

- [ ] 4.1 Crear `services/app-onboarding/` (Go) con API REST y emisión de eventos.
- [ ] 4.2 Modelo de datos: `repo_template`, `repo_template_version`, `app_onboarding_request`, `app_onboarding_event`.
- [ ] 4.3 Endpoint `POST /v1/onboarding` con flujo: validar policy → invocar scaffolder → invocar GitHub MCP → aplicar branch protections → publicar pipelines → registrar asset.
- [ ] 4.4 Endpoint `GET /v1/onboarding/{id}` con estado en vivo y stream SSE.
- [ ] 4.5 Audit completo y emisión de `app.onboarding.requested.v1`, `app.onboarding.completed.v1`, `repo.created.v1`, `branch_protection.applied.v1`.
- [ ] 4.6 Tests E2E con org GitHub de pruebas.

## 5. CI baseline y gates

- [ ] 5.1 Definir thresholds por criticidad (cobertura, severidad, licencias) como policy YAML.
- [ ] 5.2 Implementar `services/policy-engine/` extension para evaluar `pipeline.gate.evaluated.v1` events.
- [ ] 5.3 Implementar action `forge/openspec-link` que valida link y emite check status.
- [ ] 5.4 Implementar firmado y attestation con Cosign keyless (OIDC GitHub).
- [ ] 5.5 Configurar Rekor privado para repos `confidential/restricted`.
- [ ] 5.6 Publicar a Artifact Registry con tags inmutables y retención por policy.
- [ ] 5.7 Modelo de datos: `pipeline_gate_result`, `image_signature`, `sbom_record`.

## 6. PR ↔ OpenSpec linking

- [ ] 6.1 Implementar parser de `OpenSpec: <id>` en PR description.
- [ ] 6.2 Endpoint en `services/openspec/` para validar existencia/estado del OpenSpec.
- [ ] 6.3 Webhook GitHub `pull_request` → escribe `pr_openspec_link` y emite `pr.linked-to-openspec.v1`.
- [ ] 6.4 Webhook `pull_request.closed` (merged) → extender `decision_log` del OpenSpec con SHA/URL.
- [ ] 6.5 Check obligatoria `forge/openspec-link` con override gated por `criticality`.

## 7. Override / break-glass

- [ ] 7.1 Definir policy templates: `bypass-gate`, `merge-without-openspec`, `relax-branch-protection`.
- [ ] 7.2 Integrar overrides con Approvals Inbox (Fase 1) y TTL (máx 24h).
- [ ] 7.3 Auditar y emitir `policy.override.granted.v1` con datos completos del actor y razón.
- [ ] 7.4 Job de auto-revert al expirar TTL.

## 8. Portal — UI de onboarding

- [ ] 8.1 Wizard "New App": pasos template → params → preview → confirm.
- [ ] 8.2 Vista "Onboarding history" con estado en vivo y eventos.
- [ ] 8.3 Vista "Templates" con catálogo, lifecycle y trust level.
- [ ] 8.4 Vista "PR gates" con resultados por stage y links a logs.
- [ ] 8.5 Tests E2E con Playwright cubriendo el flujo completo.

## 9. Registro como asset

- [ ] 9.1 Al completar onboarding, crear asset `type=application` con metadata completa.
- [ ] 9.2 Hooks de transición de lifecycle: `proposed → in_review` (primer pipeline verde + imagen firmada), `in_review → approved` (approval Workspace owner).
- [ ] 9.3 Tests de integración Registry + onboarding.

## 10. Observabilidad y métricas

- [ ] 10.1 Métricas: `onboarding_duration_seconds`, `onboarding_success_rate`, `pipeline_gate_failure_rate`, `pr_openspec_link_coverage`, `image_signing_rate`, `override_count`.
- [ ] 10.2 Dashboards Grafana por Workspace y global.
- [ ] 10.3 SLOs iniciales: onboarding p95 ≤ 5 min, signing rate = 100% en `main`, openspec link coverage ≥ 95% en `criticality≥medium`.

## 11. Validación y sign-off

- [ ] 11.1 Ejecutar 3 onboardings de prueba en Workspace piloto (un template por categoría).
- [ ] 11.2 Verificar audit completo, eventos emitidos, assets registrados, imágenes firmadas, OpenSpec linkeado.
- [ ] 11.3 Sign-off por Platform Engineering, Security y Workspace Owner del piloto.
- [ ] 11.4 Documentación: `docs/onboarding/`, `docs/templates/authoring.md`, `docs/ci/baseline.md`.
