## Context

Fase 2 introduce el **golden path** para nacer aplicaciones dentro de Forge. Las decisiones aquí definen cómo se materializan repos, cómo se gobierna el GitHub App, qué stages son obligatorios en CI, cómo se firman/atestiguan imágenes y cómo se enlaza cada PR con OpenSpec. La fase asume Alfred operativo (Fase 1), Registry con lifecycle/trust levels, policies/approvals y MCP framework ya disponibles.

## Constraints

- **No bypass de LiteLLM**: cualquier uso de LLM en scaffolders o linters de PR pasa por el gateway (Fase 0).
- **No mutaciones cross-Workspace**: el GitHub MCP write opera scoped al Workspace destino; OpenFGA enforce el aislamiento.
- **Templates como assets gobernados**: todo template debe estar en el Registry con `lifecycle_state=approved` y `trust_level≥T3` para ser usable en producción.
- **CI obligatorio**: un repo creado vía Forge nace con los stages obligatorios; remover stages requiere policy override aprobado.
- **Firma de imágenes obligatoria** para cualquier app que vaya a desplegarse en Fase 3; sin firma, el deploy es bloqueado.
- **PR sin OpenSpec link**: bloqueado por policy en `criticality≥medium`; en `low` permitido con warning.
- **Conventional commits + Semantic PR titles**: enforced por GitHub Actions.

## Decisions

### D2.1 — Motor de scaffolding

Se usa **Go + text/template + go-getter** para clonar templates desde `forge-templates/` (subdirectorios versionados por SemVer tag). Alternativa descartada: cookiecutter (Python) — añade dependencia runtime adicional y no se integra naturalmente con el control plane Go. Los templates declaran un `template.yaml` con: parámetros, validaciones, hooks pre/post, archivos a renderizar, capabilities requeridas.

### D2.2 — GitHub App scopes y particionado

Un **único GitHub App "Forge"** instalado a nivel organización con permisos: `contents:write`, `pull_requests:write`, `administration:write` (para branch protections), `actions:write`, `checks:write`, `metadata:read`, `members:read`. Los **tokens emitidos por el MCP son short-lived (≤10 min)** y scoped al repo objetivo via installation token. Alternativa descartada: una App por Workspace — operativamente costoso y complica federación.

### D2.3 — Stages obligatorios de CI baseline

| Stage | Tool | Falla bloquea merge |
|---|---|---|
| Lint | golangci-lint / eslint / ruff | sí |
| Build | language-native | sí |
| Unit tests + coverage ≥ threshold | language-native | sí (threshold por criticidad) |
| Secrets scan | gitleaks | sí |
| SAST | Semgrep (default) + CodeQL para `criticality≥high` | sí (severity ≥ high) |
| SCA | OSV-scanner + Trivy fs | sí (severity ≥ high) |
| SBOM | Syft → SPDX + CycloneDX | no (publicación) |
| Container scan | Trivy image | sí (severity ≥ high) |
| Sign | Cosign keyless (OIDC GitHub) | sí (en main) |
| Attest | Cosign attest SLSA provenance + SBOM | sí (en main) |
| Publish | Artifact Registry | sí (en main) |

Thresholds por criticidad (low/medium/high/critical) configurables vía policy.

### D2.4 — Firma keyless con Cosign

Se adopta **Cosign keyless** usando el OIDC token de GitHub Actions (sin gestión de claves privadas). Las firmas y attestations se almacenan en **Rekor** (transparencia pública) **o Rekor privado interno** según data classification del repo. Alternativa descartada: keys gestionadas en KMS — añade overhead operativo y rotación manual.

### D2.5 — PR ↔ OpenSpec linking

Mecanismo:
1. PR description debe contener línea `OpenSpec: <id>` (uno o más).
2. GitHub Action `forge-openspec-link` valida que el OpenSpec existe, está en estado `approved` o `in_review` y pertenece al Workspace del repo.
3. El backend escribe el link en `pr_openspec_link` y emite `pr.linked-to-openspec.v1`.
4. Una check obligatoria (`forge/openspec-link`) bloquea merge si no hay link válido (excepto `criticality=low` con override).
5. Al merge, el OpenSpec recibe un evento que extiende su `decision_log` con el SHA y URL del PR.

### D2.6 — Branch protections estándar

Aplicadas automáticamente al crear el repo:
- `main` protegida; require PR; require ≥1 review (≥2 si `criticality≥high`); dismiss stale; require review from Code Owners.
- Required status checks: todos los stages CI obligatorios + `forge/openspec-link`.
- Linear history; signed commits requeridos en `criticality≥high`.
- Restringir push directo a `main`; sólo Forge GitHub App y emergency-break-glass team.

### D2.7 — Override / break-glass

Cualquier override (saltar gate, mergear sin OpenSpec link, modificar branch protection) requiere **approval explícito** vía Approvals Inbox (Fase 1) por un actor con rol `release-manager` o `security-approver`, queda en audit log y emite `policy.override.granted.v1` con TTL (máx 24h).

### D2.8 — Asset registration del onboarding

Al completar el onboarding:
1. Se crea asset `type=application` con `lifecycle_state=proposed`.
2. Tras el primer pipeline verde y firma de imagen, transición a `in_review`.
3. Tras approval del Workspace owner, transición a `approved` (queda apto para Fase 3 deploy).
4. Metadata incluye: `template_id`, `template_version`, `runtime`, `criticality`, `data_classification`, `owners`, `repo_url`, `default_branch`, `image_repository`.

## Risks / Trade-offs

- **GitHub App con permisos amplios**: mitigado con installation tokens short-lived, audit de cada uso, separación de scopes en MCP tools y rate limiting.
- **Templates como vector supply-chain**: mitigado con trust levels, firma de tags y review obligatorio para nuevos templates.
- **Falsos positivos de scanners**: mitigado con allowlists versionadas, suppressions con expiración y reportes de FP rate por scanner.
- **Costo de CI**: mitigado con caching agresivo, ejecución condicional por path filters y reuse de actions.
- **Latencia del onboarding** (target ≤5 min end-to-end): monitoreada; si excede, el wizard muestra estado en vivo y eventos.

## Migration Plan

1. Publicar **`forge-templates/`** con primer set: `go-microservice@1.0.0`, `python-fastapi-microservice@1.0.0`, `nextjs-frontend@1.0.0`, `go-library@1.0.0`, `iac-terraform-module@1.0.0` — todos en `lifecycle_state=approved`, `trust_level=T3`.
2. Publicar **`forge-actions/`** con composite actions: `forge/lint`, `forge/test-with-coverage`, `forge/sast`, `forge/sca`, `forge/sbom`, `forge/container-scan`, `forge/cosign-sign-attest`, `forge/publish-image`, `forge/openspec-link`.
3. Desplegar `services/app-onboarding/` y `services/scaffolder/`; extender `services/mcp/github/` a write-mode detrás de feature flag.
4. Habilitar policies de onboarding en un Workspace piloto; correr 3 onboardings de prueba.
5. Habilitar para todos los Workspaces; el override break-glass queda gated por `security-approver`.

## Open Questions

- ¿Soportar GitLab/Bitbucket adapters en esta fase o diferir a Fase 5? — **Decisión inicial**: solo GitHub en Fase 2; abstracción VCS preparada en interfaces para evitar refactor mayor luego.
- ¿Rekor público vs privado por defecto? — **Decisión inicial**: privado para repos `data_classification ∈ {confidential, restricted}`; público para `internal/public`.
- ¿Coverage threshold por defecto? — **Decisión inicial**: 70% (low/medium), 80% (high), 85% (critical), configurable por policy.
