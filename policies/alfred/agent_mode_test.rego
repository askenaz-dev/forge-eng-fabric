package forge.alfred.agent_mode_test

import rego.v1

import data.forge.alfred.agent_mode

test_run_allowed_in_full_autonomy if {
    agent_mode.decision == "allow" with input as {
        "action": "alfred:agent-mode.run",
        "principal": {"sub": "user:alice", "roles": ["workspace:owner"]},
        "workspace": {"id": "ws-1", "autonomy_preset": "full-autonomy", "dock_enabled": true},
        "criticality": "medium",
    }
}

test_run_requires_approval_in_manual_prod if {
    agent_mode.decision == "requires_approval" with input as {
        "action": "alfred:agent-mode.run",
        "principal": {"sub": "user:alice", "roles": ["workspace:editor"]},
        "workspace": {"id": "ws-1", "autonomy_preset": "manual-prod", "dock_enabled": true},
        "criticality": "low",
    }
}

test_critical_always_dual_control if {
    agent_mode.decision == "requires_dual_control" with input as {
        "action": "alfred:agent-mode.run",
        "principal": {"sub": "user:alice", "roles": ["workspace:owner"]},
        "workspace": {"id": "ws-1", "autonomy_preset": "full-autonomy", "dock_enabled": true},
        "criticality": "critical",
    }
}

test_cancel_owner_allowed if {
    agent_mode.decision == "allow" with input as {
        "action": "alfred:agent-mode.cancel",
        "principal": {"sub": "user:alice", "roles": ["workspace:owner"]},
        "workspace": {"id": "ws-1", "autonomy_preset": "manual-prod", "dock_enabled": true},
        "criticality": "low",
    }
}
