#!/usr/bin/env bash
# audit-no-mocks.sh
#
# Enforces the "real data only" policy from the portal-rebrand spec:
# rejects fixture, mock, fake, lorem identifiers and demo names from the
# brand notebook fixtures (Ana Restrepo, wf_8a13*, acme/payments-svc...)
# inside portal/src/ outside whitelisted test directories.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${ROOT}/portal/src"

if [[ ! -d "${TARGET}" ]]; then
  echo "audit-no-mocks: ${TARGET} not found, nothing to audit" >&2
  exit 0
fi

# Patterns that are forbidden in shipped Portal source.
PATTERNS=(
  '\bmock_'
  '\bfixture_'
  '\bfake_'
  '\blorem '
  'Ana Restrepo'
  'wf_8a13'
  'acme/payments-svc'
  'acme/orders-api'
  'acme/identity'
  "from ['\"]\.\./\.\./\.\./design/"
  "from ['\"]\.\.\/design\/"
  "PORTAL_I18N\s*\\["
  "NAV_GROUPS\s*\\["
  "MOCK_RUNS"
  "MOCK_APPROVALS"
)

# Exclusions: tests with explicit fixtures, generated files, and the no-mocks
# documentation itself.
EXCLUDE_GLOBS=(
  --glob='!portal/tests/**'
  --glob='!portal/src/**/__tests__/**'
  --glob='!portal/src/**/__fixtures__/**'
  --glob='!portal/src/**/*.test.ts'
  --glob='!portal/src/**/*.test.tsx'
  --glob='!portal/src/**/*.spec.ts'
  --glob='!portal/src/**/*.spec.tsx'
  --glob='!portal/src/**/__generated__/**'
)

fail=0
for pat in "${PATTERNS[@]}"; do
  if hits=$(rg --no-heading --line-number --color=never "${EXCLUDE_GLOBS[@]}" "${pat}" "${TARGET}" 2>/dev/null); then
    echo "audit-no-mocks: forbidden pattern '${pat}' found in portal/src/:" >&2
    echo "${hits}" >&2
    fail=1
  fi
done

if [[ "${fail}" -ne 0 ]]; then
  echo "" >&2
  echo "Portal must use real data sources only — see openspec/changes/forge-portal-rebranding/" >&2
  exit 1
fi

echo "audit-no-mocks: OK"
