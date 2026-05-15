#!/usr/bin/env bash
# build-alfred-bundle.sh — Build, sign, and publish the alfred OPA bundle.
#
# The bundle hash is stored in .bundle-hash and used by platform-ops / triager
# to stamp every audit row's policy_bundle_hash field.
#
# Usage:
#   tools/policy/build-alfred-bundle.sh [--push]
#
# Env:
#   OPA_BUNDLE_REGISTRY  e.g. ghcr.io/forge-eng-fabric/opa-bundles
#   OPA_SIGNING_KEY      path to ECDSA P-256 private key (PEM)
#   OPA_BUNDLE_TAG       defaults to git short SHA

set -euo pipefail

POLICY_DIR="policies/alfred"
BUNDLE_NAME="alfred"
PUSH="${1:-}"
OPA="${OPA_BIN:-opa}"
REGISTRY="${OPA_BUNDLE_REGISTRY:-ghcr.io/forge-eng-fabric/opa-bundles}"
TAG="${OPA_BUNDLE_TAG:-$(git rev-parse --short HEAD 2>/dev/null || echo dev)}"
SIGNING_KEY="${OPA_SIGNING_KEY:-}"
BUNDLE_FILE="dist/${BUNDLE_NAME}-${TAG}.tar.gz"
HASH_FILE=".bundle-hash"

mkdir -p dist

echo "==> lattice-check"
bash tools/policy/lattice-check.sh "${POLICY_DIR}"

echo "==> opa check (compile)"
"$OPA" check "${POLICY_DIR}" --strict

echo "==> opa build"
BUILD_ARGS=("--bundle" "${POLICY_DIR}" "--output" "${BUNDLE_FILE}")
if [[ -n "${SIGNING_KEY}" ]]; then
  BUILD_ARGS+=("--signing-key" "${SIGNING_KEY}" "--signing-alg" "ES256")
fi
"$OPA" build "${BUILD_ARGS[@]}"

# Compute SHA-256 hash of the bundle tarball.
HASH=$(sha256sum "${BUNDLE_FILE}" | awk '{print $1}')
echo "${HASH}" > "${HASH_FILE}"
echo "==> bundle hash: ${HASH}"

if [[ "${PUSH}" == "--push" ]]; then
  echo "==> pushing to ${REGISTRY}/${BUNDLE_NAME}:${TAG}"
  oras push \
    "${REGISTRY}/${BUNDLE_NAME}:${TAG}" \
    "${BUNDLE_FILE}:application/vnd.oci.image.manifest.v1+json"
  echo "==> tagging :latest"
  oras tag \
    "${REGISTRY}/${BUNDLE_NAME}:${TAG}" \
    "${REGISTRY}/${BUNDLE_NAME}:latest"
fi

echo "==> done. bundle: ${BUNDLE_FILE}, hash: ${HASH}"
