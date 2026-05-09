## Why

Con Alfred operativo y el Registry productivo (Fase 1), Forge debe permitir **onboardear aplicaciones reales** dentro de Workspaces: crear repositorios desde templates aprobados, aplicar CODEOWNERS, configurar pipelines de CI con gates de calidad/seguridad/SBOM/firmado, y dejar trazabilidad bidireccional **PR ↔ OpenSpec**. Sin esto, Alfred no tiene un sustrato consistente sobre el cual generar/modificar código de forma segura, ni la organización tiene un golden path estandarizado.

## What Changes

- **NUEVO**: **GitHub MCP** con scopes de **escritura** (crear repos, ramas, PRs, archivos, settings, branch protections) gobernados por policies y approvals.
- **NUEVO**: Servicio **scaffolding** que materializa **Repo Templates** versionados (microservicio Go, microservicio Python/FastAPI, frontend Next.js, biblioteca, infra-as-code) registrados como assets en el Registry.
- **NUEVO**: Flujo de **Onboarding de App** en el Portal: selección de template, parámetros (nombre, owners, runtime, criticidad, data classification), preview del repo a crear, ejecución con audit y eventos.
- **NUEVO**: Generación automática de **CODEOWNERS**, **branch protections**, **PR templates**, **issue templates** y **README** con links a OpenSpec y Workspace.
- **NUEVO**: **Pipelines CI iniciales** (GitHub Actions reutilizables) con stages: lint, build, unit tests, **SAST** (Semgrep/CodeQL), **SCA** (OSV/Trivy), **SBOM** (Syft), **container scan** (Trivy/Grype), **firmado de imágenes con Cosign** (keyless via OIDC) y publicación a registry interno.
- **NUEVO**: **Quality/Security gates** en PRs: cobertura mínima, severidad máxima, licencias permitidas, ausencia de secretos (gitleaks), todo configurable por criticidad de la app.
- **NUEVO**: **PR ↔ OpenSpec linker**: cada PR creado por Alfred o humanos referencia uno o más OpenSpecs; el merge sin link es bloqueado por policy salvo override aprobado.
- **NUEVO**: Registro de cada app onboardeada como **asset tipo `application`** con metadata (template_version, runtime, criticality, data_classification, owners, repo_url) en el Registry.
- **NUEVO**: Eventos `app.onboarding.*`, `repo.created.v1`, `pipeline.gate.evaluated.v1`, `pr.linked-to-openspec.v1`, `image.signed.v1`, `sbom.published.v1`.
- **MODIFICADO**: `mcp-and-skills` — extender el GitHub MCP de Fase 1 (read-only) a **read/write** con tool catalog ampliado y guardrails específicos para mutaciones de repos.
- **MODIFICADO**: `policies-and-approvals` — añadir policy templates específicos para onboarding (creación de repos, override de branch protection, override de gates, merge sin OpenSpec).
- **Criterio de salida (E2E)**: Alfred (o un humano) crea una app desde un template aprobado, el repo nace con CODEOWNERS/branch protections/pipelines, un PR de bootstrap pasa todos los gates, la imagen se firma y publica con SBOM, y el asset queda registrado y vinculado a un OpenSpec.

## Capabilities

### New Capabilities

- `app-onboarding-service`: orquestación end-to-end del alta de aplicaciones (selección de template, preview, ejecución, audit, registro como asset).
- `repo-template-catalog`: catálogo versionado de templates de repositorio (microservicios, frontend, librerías, IaC) gestionados como assets con lifecycle y trust level.
- `github-app-provisioning`: provisión gobernada de repos, CODEOWNERS, branch protections, PR/issue templates y settings, con audit completo.
- `ci-pipeline-baseline`: pipelines reutilizables con stages obligatorios (lint, build, test, SAST, SCA, SBOM, container scan, sign, publish) y gates configurables.
- `supply-chain-attestations`: SBOM (Syft), firmado Cosign keyless OIDC, attestations SLSA y verificación de imágenes en deploy.
- `pr-openspec-linking`: vinculación bidireccional PR ↔ OpenSpec con enforcement por policy y métricas de cobertura.

### Modified Capabilities

- `mcp-and-skills`: extender GitHub MCP a read/write con tool catalog completo (create_repo, create_branch, create_pr, set_branch_protection, set_codeowners, etc.) y guardrails específicos.
- `policies-and-approvals`: añadir policy templates de onboarding y overrides asociados (creación de repos, merge sin OpenSpec, bypass de gates).

## Impact

- **Servicios nuevos**: `services/app-onboarding/` (Go), `services/scaffolder/` (Go con motor de templates tipo cookiecutter/yeoman), extensión de `services/mcp/github/` a write-mode, librería `pkg/ci-templates/` con workflows reutilizables de GitHub Actions.
- **Repositorios meta**: `forge-templates/` (templates de repo versionados), `forge-actions/` (composite/reusable actions de CI).
- **Datos**: tablas `repo_template`, `repo_template_version`, `app_onboarding_request`, `app_onboarding_event`, `pipeline_gate_result`, `pr_openspec_link`, `image_signature`, `sbom_record`.
- **Eventos**: `app.onboarding.requested.v1`, `app.onboarding.completed.v1`, `repo.created.v1`, `branch_protection.applied.v1`, `pipeline.gate.evaluated.v1`, `image.signed.v1`, `sbom.published.v1`, `pr.linked-to-openspec.v1`.
- **Integraciones**: GitHub (App con scopes ampliados), GitHub Actions, Cosign + Sigstore (Fulcio/Rekor), registry de imágenes interno (Artifact Registry), escáneres (Semgrep, OSV, Trivy, Syft, gitleaks).
- **Portal**: módulo "New App" con wizard, vista de Onboarding history, vista de Templates, vista de Pipeline gates por PR.
- **Riesgos**: privilegios excesivos del GitHub App, supply-chain (templates comprometidos → mitigado por trust level y firma), falsos positivos de scanners bloqueando entregas.
- **Out of scope**: despliegue real en runtimes (Fase 3), capabilities por fase de SDLC con Jira/Confluence (Fase 4), workflows visuales (Fase 5), self-healing (Fase 6).
