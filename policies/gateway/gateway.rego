# Skill gateway authorization rules.
#
# Evaluated by services/policy-engine before the gateway lets a PAT list,
# install or invoke an asset. The package is intentionally narrow: it does
# NOT replace the gateway's hard-coded eligibility checks (approved + T1+ +
# distribution.gateway_published) — those are non-negotiable invariants. The
# policy here is the per-tenant overlay that Workspace owners and the SDLC
# Team can adjust without redeploying the gateway.

package forge.gateway

# Inputs (the gateway passes these in the OPA input bag):
#   input.action      "list" | "install" | "invoke"
#   input.developer   { sub, tenant_id, workspace_id, scopes[], allowlist[] }
#   input.asset       { id, type, trust_level, lifecycle_state, visibility }
#   input.context     { environment, data_classification }

default list_allowed     := false
default install_allowed  := false
default invoke_allowed   := false

# --- list ---------------------------------------------------------------
# Listing is permitted when the PAT carries any gateway scope and the asset
# is gateway-eligible at the registry level. Tenant scoping is enforced in
# the gateway service.

list_allowed if {
    some scope in input.developer.scopes
    scope == "gateway.read"
}

list_allowed if {
    some scope in input.developer.scopes
    scope == "gateway.install"
}

list_allowed if {
    some scope in input.developer.scopes
    scope == "gateway.invoke"
}

# --- install ------------------------------------------------------------

install_allowed if {
    list_allowed
    some scope in input.developer.scopes
    scope == "gateway.install"
    asset_in_allowlist
    asset_is_publishable
}

# --- invoke -------------------------------------------------------------

invoke_allowed if {
    install_allowed
    some scope in input.developer.scopes
    scope == "gateway.invoke"
    not_restricted_by_classification
}

# --- helpers ------------------------------------------------------------

asset_in_allowlist if {
    count(input.developer.allowlist) == 0
}

asset_in_allowlist if {
    some allowed in input.developer.allowlist
    allowed == input.asset.id
}

asset_is_publishable if {
    input.asset.lifecycle_state == "approved"
    input.asset.trust_level != "T0"
}

# Restricted data must not leave the platform via gateway invocations.
# data_classification "confidential" / "restricted" require an in-platform
# runtime; gateway invocations are refused.
not_restricted_by_classification if {
    not input.context.data_classification == "restricted"
    not input.context.data_classification == "confidential"
}
