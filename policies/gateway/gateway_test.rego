package forge.gateway

# Unit tests for the gateway policy. Run with `opa test policies/gateway`.

approved_asset := {"id": "skill:ws:foo", "type": "skill", "trust_level": "T2", "lifecycle_state": "approved", "visibility": "workspace"}
proposed_asset := {"id": "skill:ws:bar", "type": "skill", "trust_level": "T2", "lifecycle_state": "proposed", "visibility": "workspace"}

reader_pat := {"sub": "u1", "tenant_id": "t1", "workspace_id": "w1", "scopes": ["gateway.read"], "allowlist": []}
installer_pat := {"sub": "u1", "tenant_id": "t1", "workspace_id": "w1", "scopes": ["gateway.read", "gateway.install"], "allowlist": []}
invoker_pat := {"sub": "u1", "tenant_id": "t1", "workspace_id": "w1", "scopes": ["gateway.read", "gateway.install", "gateway.invoke"], "allowlist": []}

internal_ctx := {"environment": "prod", "data_classification": "internal"}
restricted_ctx := {"environment": "prod", "data_classification": "restricted"}

test_reader_can_list if {
    list_allowed with input as {"action": "list", "developer": reader_pat, "asset": approved_asset, "context": internal_ctx}
}

test_reader_cannot_install if {
    not install_allowed with input as {"action": "install", "developer": reader_pat, "asset": approved_asset, "context": internal_ctx}
}

test_installer_can_install_approved if {
    install_allowed with input as {"action": "install", "developer": installer_pat, "asset": approved_asset, "context": internal_ctx}
}

test_installer_cannot_install_proposed if {
    not install_allowed with input as {"action": "install", "developer": installer_pat, "asset": proposed_asset, "context": internal_ctx}
}

test_invoker_can_invoke_internal if {
    invoke_allowed with input as {"action": "invoke", "developer": invoker_pat, "asset": approved_asset, "context": internal_ctx}
}

test_invoker_blocked_on_restricted_data if {
    not invoke_allowed with input as {"action": "invoke", "developer": invoker_pat, "asset": approved_asset, "context": restricted_ctx}
}

test_allowlist_blocks_off_list_asset if {
    pat := {"sub": "u1", "tenant_id": "t1", "workspace_id": "w1", "scopes": ["gateway.install"], "allowlist": ["skill:ws:other"]}
    not install_allowed with input as {"action": "install", "developer": pat, "asset": approved_asset, "context": internal_ctx}
}
