package forge.gateway.a2a_test

import data.forge.gateway.a2a

test_internal_allow_default if {
    a2a.allow with input as {
        "action": "a2a.task",
        "provenance": "internal",
        "tenant_id": "t1",
        "asset_id": "forge-architect",
        "task_type": "tasks/send",
    }
}

test_external_without_allowlist_denied if {
    not a2a.allow with input as {
        "action": "a2a.task",
        "provenance": "external",
        "tenant_id": "t1",
        "asset_id": "partner-x",
        "task_type": "tasks/send",
        "task_allowlist": [],
    }
}

test_inbound_unenrolled_partner_denied if {
    not a2a.allow with input as {
        "action": "a2a.task",
        "provenance": "external_inbound",
        "principal_kind": "external_agent",
        "tenant_id": "t1",
        "partner_enrolled": false,
        "asset_in_partner_allowlist": true,
        "asset_id": "forge-architect",
    }
}

test_inbound_enrolled_partner_with_allowed_asset_allowed if {
    a2a.allow with input as {
        "action": "a2a.task",
        "provenance": "external_inbound",
        "principal_kind": "external_agent",
        "tenant_id": "t1",
        "partner_enrolled": true,
        "asset_in_partner_allowlist": true,
        "asset_id": "forge-architect",
    }
}

test_inbound_enrolled_partner_with_disallowed_asset_denied if {
    not a2a.allow with input as {
        "action": "a2a.task",
        "provenance": "external_inbound",
        "principal_kind": "external_agent",
        "tenant_id": "t1",
        "partner_enrolled": true,
        "asset_in_partner_allowlist": false,
        "asset_id": "forge-architect",
    }
}

test_cross_tenant_denied if {
    not a2a.allow with input as {
        "action": "a2a.task",
        "provenance": "internal",
        "tenant_id": "t1",
        "asset_tenant_id": "t2",
        "asset_id": "x",
        "task_type": "tasks/send",
    }
}
