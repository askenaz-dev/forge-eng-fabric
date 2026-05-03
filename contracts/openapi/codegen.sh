#!/usr/bin/env bash
# Generate Phase 0 OpenAPI clients for CI validation.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${ROOT}/contracts/generated"

rm -rf "${OUT}"
mkdir -p "${OUT}/go" "${OUT}/python" "${OUT}/typescript"

npx -y @openapitools/openapi-generator-cli generate \
  -i "${ROOT}/contracts/openapi/control-plane.yaml" \
  -g go \
  -o "${OUT}/go/control-plane" \
  --additional-properties=packageName=controlplaneclient,isGoSubmodule=true \
  --skip-validate-spec

npx -y @openapitools/openapi-generator-cli generate \
  -i "${ROOT}/contracts/openapi/registry.yaml" \
  -g go \
  -o "${OUT}/go/registry" \
  --additional-properties=packageName=registryclient,isGoSubmodule=true \
  --skip-validate-spec

npx -y @openapitools/openapi-generator-cli generate \
  -i "${ROOT}/contracts/openapi/control-plane.yaml" \
  -g python \
  -o "${OUT}/python/control_plane" \
  --additional-properties=packageName=forge_control_plane_client \
  --skip-validate-spec

npx -y @openapitools/openapi-generator-cli generate \
  -i "${ROOT}/contracts/openapi/registry.yaml" \
  -g python \
  -o "${OUT}/python/registry" \
  --additional-properties=packageName=forge_registry_client \
  --skip-validate-spec

npx -y @openapitools/openapi-generator-cli generate \
  -i "${ROOT}/contracts/openapi/control-plane.yaml" \
  -g typescript-fetch \
  -o "${OUT}/typescript/control-plane" \
  --additional-properties=npmName=@forge/control-plane-client,typescriptThreePlus=true \
  --skip-validate-spec

npx -y @openapitools/openapi-generator-cli generate \
  -i "${ROOT}/contracts/openapi/registry.yaml" \
  -g typescript-fetch \
  -o "${OUT}/typescript/registry" \
  --additional-properties=npmName=@forge/registry-client,typescriptThreePlus=true \
  --skip-validate-spec

find "${OUT}" -type f | sort
