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
for url in "${KC}/health/ready" "${CP}/healthz" "${RG}/healthz" "${AU}/healthz" "${ALFRED}/healthz"; do
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

step "Login as alice via Keycloak (forge-cli direct grant)"
TOKEN=$(curl -sf -X POST "${KC}/realms/forge/protocol/openid-connect/token" \
  -H "content-type: application/x-www-form-urlencoded" \
  -d "client_id=forge-cli" \
  -d "username=alice" \
  -d "password=alice" \
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
  -d '{"name":"hello-forge","description":"smoke test workspace","owners":["alice"]}')
WS_ID=$(echo "$WS" | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "workspace=${WS_ID}"

step "Register asset"
ASSET=$(curl -sf -X POST "${RG}/v1/workspaces/${WS_ID}/assets" "${AUTH[@]}" \
  -H "content-type: application/json" \
  -d '{"type":"prompt","name":"hello-prompt","version":"0.1.0","owners":["alice"],"metadata":{"text":"hi"}}')
ASSET_ID=$(echo "$ASSET" | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "asset=${ASSET_ID}"

step "Call Alfred /list-workspaces"
curl -sf "${ALFRED}/list-workspaces" "${AUTH[@]}" | python -m json.tool

step "Query audit"
sleep 2 # allow consumer to drain
curl -sf "${AU}/v1/audit?tenant_id=${TENANT_ID}" "${AUTH[@]}" | python -m json.tool

echo
echo "SMOKE OK"
