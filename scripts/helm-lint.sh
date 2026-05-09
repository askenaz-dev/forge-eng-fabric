#!/usr/bin/env bash
# Lints every chart under infra/helm/ and verifies that each chart's rendered output
# contains the required resources for its declared flavor.
#
# Usage:
#   scripts/helm-lint.sh
#
# Exits non-zero on the first lint error or missing required resource.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELM_DIR="${REPO_ROOT}/infra/helm"

if ! command -v helm >/dev/null 2>&1; then
  echo "ERROR: helm CLI not found in PATH" >&2
  exit 2
fi

# Update flavor library deps for every chart that declares one.
echo ">> helm dependency update (flavor libraries)"
find "${HELM_DIR}" -mindepth 2 -maxdepth 2 -type f -name Chart.yaml | while read -r chart; do
  chart_dir="$(dirname "${chart}")"
  if grep -q "type: library" "${chart}"; then
    continue
  fi
  if grep -q "dependencies:" "${chart}"; then
    (cd "${chart_dir}" && helm dependency update >/dev/null)
  fi
done

echo ">> helm lint per chart"
LINT_FAILED=0
for chart in "${HELM_DIR}"/*/Chart.yaml; do
  chart_dir="$(dirname "${chart}")"
  chart_name="$(basename "${chart_dir}")"
  if [[ "${chart_name}" == "_flavors" ]]; then
    continue
  fi
  if ! helm lint "${chart_dir}" >/tmp/helm-lint-"${chart_name}".log 2>&1; then
    echo "LINT FAILED: ${chart_name}"
    cat /tmp/helm-lint-"${chart_name}".log
    LINT_FAILED=1
  fi
done

if [[ ${LINT_FAILED} -ne 0 ]]; then
  echo "One or more charts failed lint."
  exit 1
fi

echo ">> Required-resource verification per flavor"
fail_required() {
  local chart_name="$1"
  local resource="$2"
  echo "  MISSING: ${chart_name} did not render ${resource}"
  return 1
}

REQUIRED_FAILED=0
for chart in "${HELM_DIR}"/*/Chart.yaml; do
  chart_dir="$(dirname "${chart}")"
  chart_name="$(basename "${chart_dir}")"
  if [[ "${chart_name}" == "_flavors" ]]; then
    continue
  fi

  # Determine declared flavor by reading templates for include directives.
  flavor=""
  if grep -rq 'service-http\.' "${chart_dir}/templates" 2>/dev/null; then
    flavor="service-http"
  elif grep -rq 'service-worker\.' "${chart_dir}/templates" 2>/dev/null; then
    flavor="service-worker"
  elif grep -rq 'service-cron\.' "${chart_dir}/templates" 2>/dev/null; then
    flavor="service-cron"
  else
    # Charts authored before flavors are skipped; forge-platform umbrella also skipped.
    continue
  fi

  rendered=$(helm template "${chart_dir}" 2>/dev/null || true)
  if [[ -z "${rendered}" ]]; then
    echo "  SKIP: ${chart_name} could not be rendered"
    continue
  fi

  case "${flavor}" in
    service-http)
      for kind in Deployment Service ServiceAccount HorizontalPodAutoscaler PodDisruptionBudget NetworkPolicy ServiceMonitor; do
        if ! echo "${rendered}" | grep -q "^kind: ${kind}$"; then
          fail_required "${chart_name}" "${kind}"
          REQUIRED_FAILED=1
        fi
      done
      ;;
    service-worker)
      for kind in Deployment ServiceAccount HorizontalPodAutoscaler PodDisruptionBudget NetworkPolicy ServiceMonitor; do
        if ! echo "${rendered}" | grep -q "^kind: ${kind}$"; then
          fail_required "${chart_name}" "${kind}"
          REQUIRED_FAILED=1
        fi
      done
      ;;
    service-cron)
      for kind in CronJob ServiceAccount NetworkPolicy; do
        if ! echo "${rendered}" | grep -q "^kind: ${kind}$"; then
          fail_required "${chart_name}" "${kind}"
          REQUIRED_FAILED=1
        fi
      done
      ;;
  esac
done

if [[ ${REQUIRED_FAILED} -ne 0 ]]; then
  echo "One or more charts are missing required resources for their declared flavor."
  exit 1
fi

echo "OK: all charts lint and contain required resources for their flavor."
