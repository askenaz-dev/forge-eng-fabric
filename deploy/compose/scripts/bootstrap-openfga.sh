#!/usr/bin/env bash
# Bootstraps the OpenFGA store + authorization model from contracts/openfga/authorization-model.json.
# Writes resulting IDs to deploy/compose/data/openfga.env.
set -euo pipefail

OPENFGA_API="${OPENFGA_API:-http://localhost:8088}"
MODEL_FILE="${MODEL_FILE:-contracts/openfga/authorization-model.json}"
OUT_FILE="${OUT_FILE:-deploy/compose/data/openfga.env}"

mkdir -p "$(dirname "$OUT_FILE")"

echo "[openfga] waiting for ${OPENFGA_API} ..."
for i in $(seq 1 60); do
  if curl -sf "${OPENFGA_API}/healthz" >/dev/null; then break; fi
  sleep 1
done

echo "[openfga] creating store 'forge' ..."
STORE_ID=$(curl -sf -X POST "${OPENFGA_API}/stores" \
  -H "content-type: application/json" \
  -d '{"name":"forge"}' | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')

echo "[openfga] writing authorization model ..."
MODEL_ID=$(curl -sf -X POST "${OPENFGA_API}/stores/${STORE_ID}/authorization-models" \
  -H "content-type: application/json" \
  --data-binary "@${MODEL_FILE}" | python -c 'import sys,json;print(json.load(sys.stdin)["authorization_model_id"])')

cat >"${OUT_FILE}" <<EOF
OPENFGA_STORE_ID=${STORE_ID}
OPENFGA_AUTHORIZATION_MODEL_ID=${MODEL_ID}
OPENFGA_API_URL=${OPENFGA_API}
EOF

echo "[openfga] store=${STORE_ID} model=${MODEL_ID}"
echo "[openfga] wrote ${OUT_FILE}"
