# MCP-gateway authorization rules. Loaded by services/policy-engine and
# evaluated by services/mcp-gateway on every POST /v1/gw/mcp/{asset_id}
# tool call. The Rego here is the source of truth for "who can invoke
# which tool"; the gateway itself enforces orthogonal invariants
# (registry lifecycle = approved, identity-signed headers, cross-tenant).
#
# Policy contract (input.action = "mcp.invoke"):
#
#   input.principal       string  — e.g. "user:alice", "service:runtime"
#   input.tenant_id       string
#   input.workspace_id    string
#   input.asset_id        string  — registry asset id of the MCP
#   input.tool_name       string  — e.g. "create_pr"
#   input.provenance      string  — "internal" | "external"
#   input.correlation_id  string
#
# Outputs:
#   allow                 bool
#   reason                string  — surfaced to the caller as 403 body
#
# The package is intentionally permissive by default for internal MCPs
# (they have already been through the registry lifecycle) and strict for
# external MCPs (they require an explicit Tenant allowlist).

package forge.gateway.mcp

default allow := false
default reason := "default_deny"

# --- allow paths --------------------------------------------------------

# Internal MCPs that pass the registry's `approved` precondition are
# allowed at the policy layer; the gateway still applies budget, rate
# limit and identity-signature checks before dispatch.
allow if {
    input.action == "mcp.invoke"
    input.provenance == "internal"
    not deny_cross_tenant
    not deny_classification
}

# External MCPs require the asset to be in the Tenant's allowlist (which
# the registry resolves from external_mcp_endpoint.allowlist), and the
# requested tool must be in the asset-level allowlist.
allow if {
    input.action == "mcp.invoke"
    input.provenance == "external"
    not deny_cross_tenant
    not deny_classification
    not deny_external_without_allowlist
    not deny_tool_not_allowlisted
}

# --- reasons ------------------------------------------------------------

reason := "cross_tenant_denied" if deny_cross_tenant
reason := "classification_denied" if deny_classification
reason := "external_without_allowlist" if deny_external_without_allowlist
reason := "tool_not_allowlisted" if deny_tool_not_allowlisted

# --- deny predicates ----------------------------------------------------

# Refuse any call whose asset id is registered under a different Tenant.
# The registry already enforces this on read; the policy layer catches
# the case where the caller is impersonating cross-tenant via header
# spoofing — defense in depth.
deny_cross_tenant if {
    input.asset_tenant_id != ""
    input.asset_tenant_id != input.tenant_id
}

# Restricted/confidential data classifications never leave the platform
# via an external MCP, even when the Tenant has an allowlist for it.
deny_classification if {
    input.provenance == "external"
    some forbidden in {"confidential", "restricted"}
    input.data_classification == forbidden
}

# An external MCP without an allowlist on the asset row is deny-by-default
# at policy. The registry stores the per-Tenant allowlist on
# external_mcp_endpoint; the gateway forwards it on `input.tool_allowlist`.
deny_external_without_allowlist if {
    input.provenance == "external"
    not input.tool_allowlist
}

deny_external_without_allowlist if {
    input.provenance == "external"
    count(input.tool_allowlist) == 0
}

# When the asset has a non-empty tool allowlist, the requested tool MUST
# be on it. The gateway performs the same check in code; this rule lets
# operators tighten the deny path with additional Rego logic if needed.
deny_tool_not_allowlisted if {
    input.provenance == "external"
    count(input.tool_allowlist) > 0
    not tool_in_allowlist
}

tool_in_allowlist if {
    some t in input.tool_allowlist
    t == input.tool_name
}
