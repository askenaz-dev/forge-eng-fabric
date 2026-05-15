# Tenant override example: "acme".
#
# Overrides MAY only narrow the global policy — they may NEVER relax it.
# Permitted narrowing: autonomous → requires_approval
# NOT permitted:       requires_approval → autonomous
#
# The lattice-check tool (tools/policy/lattice-check.sh) rejects any
# override that relaxes a global rule at CI time.
#
# This example shows acme requiring explicit approval for all mutate-config
# actions, overriding the default autonomous decision for low-blast-radius ones.

package forge.alfred.risk_classifier.overrides.acme

# Override: acme requires approval for any config mutation regardless of
# blast_radius or reversibility.
tenant_autonomy_override := "requires_approval" if {
	input.action_class == "mutate-config"
}

# Override: acme disables autonomous mutate-runtime for tenant-scoped actions.
tenant_autonomy_override := "requires_approval" if {
	input.action_class == "mutate-runtime"
	input.blast_radius == "tenant"
}
