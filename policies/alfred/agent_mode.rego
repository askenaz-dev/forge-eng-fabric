# Alfred agent-mode authorization rules.
#
# Evaluated by services/policy-engine before Alfred starts or cancels an
# agent-mode session. The package is small by design — most of the gating
# happens upstream in OpenFGA (workspace membership) and in
# services/alfred/alfred/agent_mode/executor.py (frozen autonomy ceiling).
# These rules cover the two coarse action classes:
#
#   alfred:agent-mode.run     start a new session
#   alfred:agent-mode.cancel  cancel an in-flight session
#
# Inputs:
#   input.action      string ("alfred:agent-mode.run" | "alfred:agent-mode.cancel")
#   input.principal   { sub, roles[] }
#   input.workspace   { id, autonomy_preset, dock_enabled }
#   input.criticality "low" | "medium" | "high" | "critical"

package forge.alfred.agent_mode

default decision     := "deny"
default rationale    := "no rule matched"

# `decision` is one of: allow | requires_approval | requires_dual_control | deny.

decision := "allow" if {
    input.action == "alfred:agent-mode.run"
    input.workspace.dock_enabled
    input.workspace.autonomy_preset == "full-autonomy"
    input.criticality != "critical"
}

decision := "allow" if {
    input.action == "alfred:agent-mode.run"
    input.workspace.dock_enabled
    input.workspace.autonomy_preset == "staging-only"
    input.criticality != "critical"
}

decision := "requires_approval" if {
    input.action == "alfred:agent-mode.run"
    input.workspace.dock_enabled
    input.workspace.autonomy_preset == "manual-prod"
}

decision := "requires_dual_control" if {
    input.action == "alfred:agent-mode.run"
    input.criticality == "critical"
}

# Cancellation is always at most `requires_approval` so a stuck session can be
# stopped by the workspace owner without needing dual control.
decision := "allow" if {
    input.action == "alfred:agent-mode.cancel"
    input.workspace.dock_enabled
    some role in input.principal.roles
    role == "workspace:owner"
}

decision := "requires_approval" if {
    input.action == "alfred:agent-mode.cancel"
    input.workspace.dock_enabled
    not some_role_owner
}

some_role_owner if {
    some role in input.principal.roles
    role == "workspace:owner"
}

rationale := "agent-mode disabled for workspace" if {
    not input.workspace.dock_enabled
}
