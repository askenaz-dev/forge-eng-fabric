package forge.gateway.mcp_test

import data.forge.gateway.mcp

test_internal_allow_default if {
    mcp.allow with input as {
        "action": "mcp.invoke",
        "provenance": "internal",
        "tenant_id": "t1",
        "workspace_id": "ws1",
        "asset_id": "github",
        "tool_name": "create_pr",
    }
}

test_external_without_allowlist_denied if {
    not mcp.allow with input as {
        "action": "mcp.invoke",
        "provenance": "external",
        "tenant_id": "t1",
        "asset_id": "vendor-x",
        "tool_name": "list_docs",
        "tool_allowlist": [],
    }
}

test_external_with_matching_allowlist_allowed if {
    mcp.allow with input as {
        "action": "mcp.invoke",
        "provenance": "external",
        "tenant_id": "t1",
        "asset_id": "vendor-x",
        "tool_name": "list_docs",
        "tool_allowlist": ["list_docs", "read_doc"],
    }
}

test_external_with_non_matching_tool_denied if {
    not mcp.allow with input as {
        "action": "mcp.invoke",
        "provenance": "external",
        "tenant_id": "t1",
        "asset_id": "vendor-x",
        "tool_name": "delete_everything",
        "tool_allowlist": ["read_doc"],
    }
}

test_cross_tenant_denied if {
    not mcp.allow with input as {
        "action": "mcp.invoke",
        "provenance": "internal",
        "tenant_id": "t1",
        "asset_tenant_id": "t2",
        "asset_id": "github",
        "tool_name": "create_pr",
    }
}

test_external_with_restricted_classification_denied if {
    not mcp.allow with input as {
        "action": "mcp.invoke",
        "provenance": "external",
        "tenant_id": "t1",
        "asset_id": "vendor-x",
        "tool_name": "read",
        "tool_allowlist": ["read"],
        "data_classification": "restricted",
    }
}
