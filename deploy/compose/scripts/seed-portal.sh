#!/usr/bin/env bash
# seed-portal.sh — Seed deterministic Portal e2e fixtures into the dev stack.
#
# Inserts a tenant + workspace + agents + skills + MCP tools + runs + approvals
# + audit events + KPI samples that the Portal Playwright suite (theme.spec,
# dashboard.spec, command-palette.spec, approvals.spec, run-sheet.spec) asserts
# against. Idempotent — running it twice is a no-op when the data already
# matches the expected shape.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"

CONTROL_PLANE_URL="${CONTROL_PLANE_URL:-http://localhost:8081}"
APPROVALS_URL="${APPROVALS_URL:-http://localhost:8105}"
SDLC_URL="${SDLC_URL:-http://localhost:8086}"
REGISTRY_URL="${REGISTRY_URL:-http://localhost:8089}"
AUDIT_URL="${AUDIT_URL:-http://localhost:8088}"
OBS_URL="${OBS_URL:-http://localhost:8091}"

# A seed token recognised by the dev keycloak realm (forge-dev/portal-e2e).
SEED_TOKEN="${SEED_TOKEN:-dev-portal-e2e-token}"

post() {
  local url="$1"; shift
  local body="$1"; shift
  curl -fsS -X POST -H "content-type: application/json" -H "authorization: Bearer ${SEED_TOKEN}" \
    "${url}" -d "${body}" >/dev/null || {
      echo "seed-portal: POST ${url} failed (continuing)" >&2
    }
}

echo ">> seed-portal: tenant + workspace"
post "${CONTROL_PLANE_URL}/v1/tenants" \
  '{"id":"acme","name":"Acme","display_name":"Acme Engineering"}'
post "${CONTROL_PLANE_URL}/v1/workspaces" \
  '{"tenant_id":"acme","business_unit_id":"engineering","name":"engineering","display_name":"Engineering","owners":["ana@acme.io"]}'

echo ">> seed-portal: registry assets (agents, skills, MCP)"
# Loaded from a deterministic JSON manifest committed at deploy/compose/seeds/portal-registry.json
if [[ -f "${ROOT}/deploy/compose/seeds/portal-registry.json" ]]; then
  post "${REGISTRY_URL}/v1/seed" "@${ROOT}/deploy/compose/seeds/portal-registry.json"
fi

echo ">> seed-portal: sdlc runs (50)"
if [[ -f "${ROOT}/deploy/compose/seeds/portal-runs.json" ]]; then
  post "${SDLC_URL}/v1/seed/runs" "@${ROOT}/deploy/compose/seeds/portal-runs.json"
fi

echo ">> seed-portal: pending approvals"
if [[ -f "${ROOT}/deploy/compose/seeds/portal-approvals.json" ]]; then
  post "${APPROVALS_URL}/v1/seed" "@${ROOT}/deploy/compose/seeds/portal-approvals.json"
fi

echo ">> seed-portal: audit events"
if [[ -f "${ROOT}/deploy/compose/seeds/portal-audit.json" ]]; then
  post "${AUDIT_URL}/v1/seed" "@${ROOT}/deploy/compose/seeds/portal-audit.json"
fi

echo ">> seed-portal: observability KPIs"
if [[ -f "${ROOT}/deploy/compose/seeds/portal-kpis.json" ]]; then
  post "${OBS_URL}/v1/seed/kpis" "@${ROOT}/deploy/compose/seeds/portal-kpis.json"
fi

echo "seed-portal: complete"
