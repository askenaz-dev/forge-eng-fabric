# Alfred self-protection denylist — D10.
#
# Non-negotiable: Alfred MUST NOT take any autonomous action whose resolved
# target is one of the protected services below. These outages MUST page humans.
#
# Admins cannot override this policy (build-time override-relaxation rejection
# is enforced by tools/policy/lattice-check.sh).
#
# Inputs:
#   input.target   string — the resolved service or principal name being acted on.

package forge.alfred.self_protection

# The canonical denylist. Entries are lower-cased service/principal names.
protected_targets := {
	"alfred",
	"symptom-triager",
	"platform-ops",
	"opa",
	"keycloak",
	"openfga",
}

# `denied` is true when the action targets a protected service.
denied if {
	some t in protected_targets
	lower(input.target) == t
}

# Also deny when the target contains a protected name as a prefix
# (e.g. "alfred-agent-mode" → denied).
denied if {
	some t in protected_targets
	startswith(lower(input.target), t)
}

# Rationale: exposed as data.forge.alfred.self_protection.denied
# and referenced by risk-classifier.rego and the guardrail layer.
