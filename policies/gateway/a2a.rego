# A2A-gateway authorization rules. Evaluated on every POST
# /v1/gw/a2a/{asset_id} (both outbound and inbound). The policy
# distinguishes the three "source" categories the spec calls out:
#
#   internal         — internal caller → internal agent (no gateway hops)
#   external_proxy   — internal caller → external A2A agent
#   inbound_external — external partner → internal Forge agent
#
# Each path has its own allow rule so operators can tighten one without
# touching the others.

package forge.gateway.a2a

default allow := false
default reason := "default_deny"

# --- allow paths --------------------------------------------------------

allow if {
    input.action == "a2a.task"
    input.provenance == "internal"
    not deny_cross_tenant
}

allow if {
    input.action == "a2a.task"
    input.provenance == "external"
    not deny_cross_tenant
    not deny_classification
    not deny_external_without_allowlist
    not deny_task_not_allowlisted
}

allow if {
    input.action == "a2a.task"
    input.provenance == "external_inbound"
    input.principal_kind == "external_agent"
    input.partner_enrolled
    input.asset_in_partner_allowlist
    not deny_cross_tenant
}

# --- reasons ------------------------------------------------------------

reason := "cross_tenant_denied" if deny_cross_tenant
reason := "classification_denied" if deny_classification
reason := "external_without_allowlist" if deny_external_without_allowlist
reason := "task_not_allowlisted" if deny_task_not_allowlisted
reason := "partner_not_enrolled" if {
    input.provenance == "external_inbound"
    not input.partner_enrolled
}
reason := "asset_not_in_partner_allowlist" if {
    input.provenance == "external_inbound"
    input.partner_enrolled
    not input.asset_in_partner_allowlist
}

# --- deny predicates ----------------------------------------------------

deny_cross_tenant if {
    input.asset_tenant_id != ""
    input.asset_tenant_id != input.tenant_id
}

deny_classification if {
    input.provenance == "external"
    some forbidden in {"confidential", "restricted"}
    input.data_classification == forbidden
}

deny_external_without_allowlist if {
    input.provenance == "external"
    not input.task_allowlist
}

deny_external_without_allowlist if {
    input.provenance == "external"
    count(input.task_allowlist) == 0
}

deny_task_not_allowlisted if {
    input.provenance == "external"
    count(input.task_allowlist) > 0
    not task_in_allowlist
}

task_in_allowlist if {
    some t in input.task_allowlist
    t == input.task_type
}
