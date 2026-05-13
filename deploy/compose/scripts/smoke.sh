#!/usr/bin/env bash
# End-to-end local smoke test for the Phase 0 stack.
# Assumes `make up` has been run and all containers are healthy.
set -euo pipefail

KC="${KC:-http://localhost:8080}"
CP="${CP:-http://localhost:8081}"
RG="${RG:-http://localhost:8082}"
AU="${AU:-http://localhost:8083}"
ALFRED="${ALFRED:-http://localhost:8090}"

step() { printf '\n=== %s ===\n' "$1"; }

step "Wait for core services"
for url in "${KC}/realms/forge/.well-known/openid-configuration" "${CP}/readyz" "${RG}/healthz" "${AU}/healthz" "${ALFRED}/readyz"; do
  for i in $(seq 1 60); do
    if curl -sf "$url" >/dev/null; then echo "ok: $url"; break; fi
    sleep 1
  done
done

step "Bootstrap OpenFGA (idempotent — skips if env file exists)"
if [[ ! -f deploy/compose/data/openfga.env ]]; then
  ./deploy/compose/scripts/bootstrap-openfga.sh
fi
# shellcheck disable=SC1091
source deploy/compose/data/openfga.env
echo "store=${OPENFGA_STORE_ID} model=${OPENFGA_AUTHORIZATION_MODEL_ID}"

step "Validate Milvus synthetic RAG collection"
if [[ "${SKIP_MILVUS_SMOKE:-0}" != "1" ]]; then
  docker run --rm --network forge-fabric_forge \
    -v "${PWD}/deploy/compose/scripts/milvus-smoke.py:/app/milvus-smoke.py:ro" \
    python:3.12-slim sh -c "pip install -q pymilvus==2.4.10 && python /app/milvus-smoke.py"
else
  echo "skipped via SKIP_MILVUS_SMOKE=1"
fi

step "Login as askenaz via Keycloak (forge-cli direct grant)"
TOKEN=$(curl -sf -X POST "${KC}/realms/forge/protocol/openid-connect/token" \
  -H "content-type: application/x-www-form-urlencoded" \
  -d "client_id=forge-cli" \
  -d "username=askenaz" \
  -d "password=askenaz" \
  -d "grant_type=password" \
  -d "scope=openid profile email" | python -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')
echo "got token (len=${#TOKEN})"
AUTH=(-H "authorization: Bearer ${TOKEN}")

step "Create tenant"
TENANT=$(curl -sf -X POST "${CP}/v1/tenants" "${AUTH[@]}" \
  -H "content-type: application/json" -d '{"name":"acme"}')
TENANT_ID=$(echo "$TENANT" | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "tenant=${TENANT_ID}"

step "Create business unit"
BU=$(curl -sf -X POST "${CP}/v1/tenants/${TENANT_ID}/business-units" "${AUTH[@]}" \
  -H "content-type: application/json" -d '{"name":"platform"}')
BU_ID=$(echo "$BU" | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "bu=${BU_ID}"

step "Create workspace"
WS=$(curl -sf -X POST "${CP}/v1/business-units/${BU_ID}/workspaces" "${AUTH[@]}" \
  -H "content-type: application/json" \
  -d '{"name":"hello-forge","description":"smoke test workspace","owners":["askenaz"]}')
WS_ID=$(echo "$WS" | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "workspace=${WS_ID}"

step "Record GitHub installation"
curl -sf -X POST "${CP}/v1/workspaces/${WS_ID}/github/installations" "${AUTH[@]}" \
  -H "content-type: application/json" \
  -d '{"installation_id":"local-installation","github_account":"forge-local","scopes":["metadata:read","contents:read"]}' | python -m json.tool

step "List GitHub repositories"
REPOS=$(curl -sf "${CP}/v1/workspaces/${WS_ID}/github/repositories" "${AUTH[@]}")
echo "${REPOS}" | python -m json.tool
REPO_COUNT=$(echo "${REPOS}" | python -c 'import sys,json;print(len(json.load(sys.stdin)["repositories"]))')
if [[ "${REPO_COUNT}" -lt 1 ]]; then
  echo "expected at least one GitHub repository"
  exit 1
fi

step "Register asset"
ASSET=$(curl -sf -X POST "${RG}/v1/workspaces/${WS_ID}/assets" "${AUTH[@]}" \
  -H "content-type: application/json" \
  -d '{"type":"prompt_template","name":"hello-prompt","version":"0.1.0","owner_team":"platform","inputs_schema":{"type":"object","properties":{"name":{"type":"string"}}},"outputs_schema":{"type":"object","properties":{"message":{"type":"string"}}},"visibility":"workspace","owners":["askenaz"],"metadata":{"text":"hi"}}')
ASSET_ID=$(echo "$ASSET" | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "asset=${ASSET_ID}"

step "Verify duplicate asset version is rejected"
DUP_STATUS=$(curl -s -o /tmp/forge-duplicate-asset.json -w "%{http_code}" -X POST "${RG}/v1/workspaces/${WS_ID}/assets" "${AUTH[@]}" \
  -H "content-type: application/json" \
  -d '{"type":"prompt_template","name":"hello-prompt","version":"0.1.0","owner_team":"platform","inputs_schema":{"type":"object"},"outputs_schema":{"type":"object"},"visibility":"workspace","owners":["askenaz"],"metadata":{"text":"hi"}}')
if [[ "${DUP_STATUS}" != "409" ]]; then
  echo "expected duplicate asset publish to return 409, got ${DUP_STATUS}"
  cat /tmp/forge-duplicate-asset.json
  exit 1
fi

step "Verify Alfred real API"
curl -sf "${ALFRED}/readyz" | python -m json.tool
curl -sf "${ALFRED}/v1/decisions?workspace_id=${WS_ID}" "${AUTH[@]}" | python -m json.tool

step "Query audit"
sleep 2 # allow consumer to drain
curl -sf "${AU}/v1/audit?tenant_id=${TENANT_ID}" "${AUTH[@]}" | python -m json.tool

step "Verify audit chain"
curl -sf "${AU}/v1/audit/verify?tenant_id=${TENANT_ID}" "${AUTH[@]}" | python -m json.tool

step "Verify audit append-only triggers"
if docker compose -f deploy/compose/docker-compose.yaml exec -T postgres psql -U forge -d forge_audit \
  -c "UPDATE audit_event SET actor='tampered' WHERE id = (SELECT id FROM audit_event WHERE tenant_id='${TENANT_ID}' LIMIT 1);" >/tmp/forge-audit-update.txt 2>&1; then
  echo "expected audit_event UPDATE to be rejected"
  cat /tmp/forge-audit-update.txt
  exit 1
else
  echo "audit_event UPDATE rejected"
fi
if docker compose -f deploy/compose/docker-compose.yaml exec -T postgres psql -U forge -d forge_audit \
  -c "DELETE FROM audit_event WHERE id = (SELECT id FROM audit_event WHERE tenant_id='${TENANT_ID}' LIMIT 1);" >/tmp/forge-audit-delete.txt 2>&1; then
  echo "expected audit_event DELETE to be rejected"
  cat /tmp/forge-audit-delete.txt
  exit 1
else
  echo "audit_event DELETE rejected"
fi

echo
echo "SMOKE OK"
