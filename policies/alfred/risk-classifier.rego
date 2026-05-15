# Alfred risk classifier — D6.
#
# Pure function: (action_class, blast_radius, reversibility, scope) →
#   (autonomy_decision, sandbox_min_tier, approvers[]).
#
# The LLM never decides whether an action is safe; this policy does.
# platform-ops calls OPA before every mutating endpoint.
# The executor calls OPA before invoking platform-ops.
# The triager calls OPA before spawning a session.
#
# Inputs:
#   input.action_class    "diagnose" | "mutate-runtime" | "mutate-data"
#                       | "mutate-config" | "mutate-code" | "mutate-infra"
#   input.blast_radius    "process" | "service" | "tenant" | "platform"
#   input.reversibility   "trivial" | "easy" | "hard" | "irreversible"
#   input.scope           "local" | "shared"
#   input.actor           string (e.g. "system:alfred" or a human principal)
#   input.tenant_overrides  object (tenant-scoped policy overrides, optional)
#   input.workspace_overrides object (workspace-scoped overrides, optional)

package forge.alfred.risk_classifier

default autonomy_decision        := "deny"
default sandbox_min_tier         := 0
default approvers                := []
default approval_mode            := "any"
default self_revoke_window_secs  := 60

# ── Diagnose: always autonomous, no sandbox needed ───────────────────────────

autonomy_decision := "autonomous" if {
	input.action_class == "diagnose"
	not _self_protected
}

sandbox_min_tier := 0 if {
	input.action_class == "diagnose"
}

# ── Mutate-runtime (restart, scale, cordon) ──────────────────────────────────

autonomy_decision := "autonomous" if {
	input.action_class == "mutate-runtime"
	input.blast_radius == "process"
	input.reversibility in {"trivial", "easy"}
	not _self_protected
	not _override_requires_approval
}

autonomy_decision := "requires_approval" if {
	input.action_class == "mutate-runtime"
	input.blast_radius in {"service", "tenant"}
	not _self_protected
}

sandbox_min_tier := 0 if {
	input.action_class == "mutate-runtime"
	input.blast_radius == "process"
}

sandbox_min_tier := 1 if {
	input.action_class == "mutate-runtime"
	input.blast_radius in {"service", "tenant"}
}

# ── Mutate-data (migrations) ──────────────────────────────────────────────────

autonomy_decision := "requires_approval" if {
	input.action_class == "mutate-data"
	not _self_protected
}

sandbox_min_tier := 1 if {
	input.action_class == "mutate-data"
}

# ── Mutate-config (feature flags, secrets) ────────────────────────────────────

autonomy_decision := "autonomous" if {
	input.action_class == "mutate-config"
	input.blast_radius == "service"
	input.reversibility in {"trivial", "easy"}
	not _self_protected
	not _override_requires_approval
}

autonomy_decision := "requires_approval" if {
	input.action_class == "mutate-config"
	input.blast_radius in {"tenant", "platform"}
	not _self_protected
}

sandbox_min_tier := 1 if {
	input.action_class == "mutate-config"
}

# ── Mutate-code (open PR) ─────────────────────────────────────────────────────

# Code PRs always require approval (Alfred never merges).
autonomy_decision := "requires_approval" if {
	input.action_class == "mutate-code"
	not _self_protected
}

sandbox_min_tier := 0 if {
	input.action_class == "mutate-code"
}

# Dual-approval: both admin and app-owner must approve code PRs.
approvers := ["admin", "app-owner"] if {
	input.action_class == "mutate-code"
}

approval_mode := "dual" if {
	input.action_class == "mutate-code"
}

# ── Mutate-infra ──────────────────────────────────────────────────────────────

autonomy_decision := "requires_dual_control" if {
	input.action_class == "mutate-infra"
	not _self_protected
}

sandbox_min_tier := 2 if {
	input.action_class == "mutate-infra"
}

approvers := ["admin", "platform-admin"] if {
	input.action_class == "mutate-infra"
}

approval_mode := "dual" if {
	input.action_class == "mutate-infra"
}

# ── Irreversible actions require dual approval ────────────────────────────────

approval_mode := "dual" if {
	input.reversibility == "irreversible"
	input.action_class != "diagnose"
}

# ── Irreversible actions are always requires_dual_control ─────────────────────

autonomy_decision := "requires_dual_control" if {
	input.reversibility == "irreversible"
	input.action_class != "diagnose"
	not _self_protected
}

# ── Platform-scope always requires dual control ───────────────────────────────

autonomy_decision := "requires_dual_control" if {
	input.blast_radius == "platform"
	input.action_class != "diagnose"
	not _self_protected
}

# ── Self-protection denylist overrides everything (see self-protection.rego) ──

autonomy_decision := "deny" if { _self_protected }

# ── Helpers ───────────────────────────────────────────────────────────────────

_self_protected if {
	# Defer to self-protection.rego via data.forge.alfred.self_protection.denied.
	data.forge.alfred.self_protection.denied
}

_override_requires_approval if {
	input.tenant_overrides.autonomy == "requires_approval"
}

_override_requires_approval if {
	input.workspace_overrides.autonomy == "requires_approval"
}

# ── Auto-fix opt-out ─────────────────────────────────────────────────────────
# Workspace, tenant, or repo (.forge/policy.yaml) can set
# alfred.auto_fix.enabled = false to deny all autonomous mutating actions.
# The opt-out does not affect diagnose actions.
# Lattice: repo_overrides narrowing applies on top of workspace/tenant.

_auto_fix_disabled if {
	input.action_class != "diagnose"
	input.workspace_overrides["alfred.auto_fix.enabled"] == false
}

_auto_fix_disabled if {
	input.action_class != "diagnose"
	input.tenant_overrides["alfred.auto_fix.enabled"] == false
}

_auto_fix_disabled if {
	input.action_class != "diagnose"
	input.repo_overrides["alfred.auto_fix.enabled"] == false
}

autonomy_decision := "deny" if { _auto_fix_disabled }

# ── Repo-level narrowing (loaded from .forge/policy.yaml) ────────────────────
# Repo policy can only make the global decision stricter (narrowing), never relax it.
# If repo_overrides.autonomy is "requires_approval", the decision is at least
# requires_approval even if the global policy would allow autonomous.

_override_requires_approval if {
	input.repo_overrides.autonomy == "requires_approval"
}

_override_requires_approval if {
	input.repo_overrides.autonomy == "requires_dual_control"
}
