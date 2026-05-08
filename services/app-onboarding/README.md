# App Onboarding Service

The app onboarding service orchestrates Forge Phase 2 repository creation from governed templates.

## Endpoints

- `POST /v1/onboarding`
- `GET /v1/onboarding`
- `GET /v1/onboarding/{id}`
- `GET /v1/onboarding/{id}/timeline`
- `GET /v1/onboarding/{id}/events`
- `GET /v1/templates`
- `GET /v1/pipeline-gates`
- `GET /metrics`

## Configuration

- `FORGE_TEMPLATES_DIR`: filesystem template catalog root.
- `FORGE_GITHUB_MCP_URL`: GitHub MCP write-mode URL.
- `FORGE_REGISTRY_URL` or `REGISTRY_URL`: Registry service URL for application asset registration.
- `FORGE_REGISTRY_TOKEN`: bearer token used when creating Registry assets.
- `ADDR`: HTTP bind address, default `:8085`.

## Flow

The service validates policy, resolves a template, renders scaffolding, invokes GitHub MCP write tools, applies protections/checks, registers an `application` asset and emits stage events plus CloudEvents.
