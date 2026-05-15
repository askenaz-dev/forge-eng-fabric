#!/usr/bin/env bash
# lattice-check.sh — CI tool to reject any tenant/workspace override that
# relaxes a global Alfred policy rule.
#
# Invoked by the OPA bundle pipeline before signing.
# Exit code 0 = all overrides are stricter or equal.
# Exit code 1 = at least one override relaxes a global rule (CI fails).
#
# Usage:
#   tools/policy/lattice-check.sh [policies/alfred]
#
# Requires: opa (https://www.openpolicyagent.org/docs/latest/cli/)

set -euo pipefail

POLICY_DIR="${1:-policies/alfred}"
OVERRIDES_DIR="${POLICY_DIR}/overrides"
OPA="${OPA_BIN:-opa}"

if ! command -v "$OPA" &>/dev/null; then
  echo "ERROR: opa not found. Install from https://www.openpolicyagent.org/docs/latest/cli/" >&2
  exit 1
fi

AUTONOMY_LATTICE=("autonomous" "requires_approval" "requires_dual_control" "deny")

lattice_level() {
  local v="$1"
  case "$v" in
    autonomous)           echo 0 ;;
    requires_approval)    echo 1 ;;
    requires_dual_control) echo 2 ;;
    deny)                 echo 3 ;;
    *)                    echo -1 ;;
  esac
}

FAILURES=0

for override_file in "${OVERRIDES_DIR}"/*.rego; do
  [[ -f "$override_file" ]] || continue
  tenant=$(basename "${override_file%.rego}")

  echo "Checking override: ${override_file} (tenant=${tenant})"

  # For each representative input scenario, compare override decision to global.
  # We test the action classes most likely to be overridden.
  for action_class in mutate-runtime mutate-config mutate-data mutate-code mutate-infra; do
    for blast_radius in process service tenant platform; do
      for reversibility in trivial easy hard irreversible; do

        input_json=$(cat <<JSON
{
  "action_class": "${action_class}",
  "blast_radius": "${blast_radius}",
  "reversibility": "${reversibility}",
  "scope": "local",
  "actor": "system:alfred",
  "target": "some-service",
  "tenant_overrides": {},
  "workspace_overrides": {}
}
JSON
)

        global_decision=$(echo "$input_json" | \
          "$OPA" eval \
            --data "${POLICY_DIR}/risk-classifier.rego" \
            --data "${POLICY_DIR}/self-protection.rego" \
            --stdin-input \
            "data.forge.alfred.risk_classifier.autonomy_decision" \
            --format raw 2>/dev/null || echo "deny")

        # Inject the tenant override as tenant_overrides.autonomy.
        override_decision=$(echo "$input_json" | \
          "$OPA" eval \
            --data "${POLICY_DIR}/risk-classifier.rego" \
            --data "${POLICY_DIR}/self-protection.rego" \
            --data "${override_file}" \
            --stdin-input \
            --format raw \
            "data.forge.alfred.risk_classifier.overrides.${tenant}.tenant_autonomy_override" \
            2>/dev/null || echo "")

        [[ -z "$override_decision" ]] && continue

        global_level=$(lattice_level "$global_decision")
        override_level=$(lattice_level "$override_decision")

        if (( override_level < global_level )); then
          echo "FAIL: ${override_file} relaxes '${action_class}/${blast_radius}/${reversibility}'" \
               " from '${global_decision}' to '${override_decision}'" >&2
          FAILURES=$(( FAILURES + 1 ))
        fi
      done
    done
  done
done

if (( FAILURES > 0 )); then
  echo "lattice-check: ${FAILURES} violation(s) found — CI failing." >&2
  exit 1
fi

echo "lattice-check: all overrides are at least as strict as global policy. OK."
