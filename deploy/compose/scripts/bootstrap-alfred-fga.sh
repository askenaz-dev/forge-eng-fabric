#!/usr/bin/env bash
# Bootstrap OpenFGA tuples for system:alfred (D12).
#
# Creates:
#   - Principal system:alfred with capability group alfred:platform-readonly
#   - Capability groups: alfred:platform-operator, alfred:tenant-operator (no grants yet)
#
# Must be run AFTER bootstrap-openfga.sh has written deploy/compose/data/openfga.env.
set -euo pipefail

source "${ENV_FILE:-deploy/compose/data/openfga.env}"

OPENFGA_API="${OPENFGA_API_URL:-http://localhost:8088}"

echo "[alfred-fga] seeding system:alfred tuples in store=${OPENFGA_STORE_ID}"

write_tuple() {
  local user="$1" relation="$2" object="$3"
  curl -sf -X POST \
    "${OPENFGA_API}/stores/${OPENFGA_STORE_ID}/write" \
    -H "content-type: application/json" \
    -d "{\"writes\":{\"tuple_keys\":[{\"user\":\"${user}\",\"relation\":\"${relation}\",\"object\":\"${object}\"}]},\"authorization_model_id\":\"${OPENFGA_AUTHORIZATION_MODEL_ID}\"}" \
    > /dev/null
  echo "[alfred-fga] wrote: ${user} ${relation} ${object}"
}

# system:alfred is a platform-level agent principal.
# Seed with platform-readonly only in iter 2; operator grants added in iter 3.
write_tuple "user:system:alfred" "member" "capability_group:alfred:platform-readonly"

echo "[alfred-fga] done."
