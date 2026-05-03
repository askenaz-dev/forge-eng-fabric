from __future__ import annotations

from datetime import datetime, timedelta

from fastapi.testclient import TestClient

from permissions.app import create_app
from permissions.store import InMemoryPermissionStore


def test_alfred_without_grant_is_denied_then_allowed_revoked_and_expired() -> None:
    store = InMemoryPermissionStore()
    app = create_app(store=store)

    check = {
        "subject": "alfred",
        "scope_kind": "workspace",
        "scope_id": "ws-1",
        "action_class": "openspec:write",
        "criticality": "medium",
    }

    with TestClient(app) as client:
        denied = client.post("/v1/permissions/check", json=check)
        assert denied.status_code == 200
        assert denied.json()["allowed"] is False

        created = client.post(
            "/v1/permissions/grants",
            json={
                "subject": "alfred",
                "scope_kind": "workspace",
                "scope_id": "ws-1",
                "action_class": "openspec:write",
                "max_criticality": "medium",
                "justification": "Let Alfred update OpenSpecs",
                "requester": "alice",
                "approver": "owner-1",
            },
        )
        assert created.status_code == 201
        grant_id = created.json()["id"]

        allowed = client.post("/v1/permissions/check", json=check)
        assert allowed.json()["allowed"] is True

        too_critical = client.post("/v1/permissions/check", json={**check, "criticality": "critical"})
        assert too_critical.json()["allowed"] is False

        revoked = client.post(
            f"/v1/permissions/grants/{grant_id}/revoke",
            json={"actor": "owner-1", "rationale": "scope complete"},
        )
        assert revoked.status_code == 200
        assert revoked.json()["status"] == "revoked"
        assert client.post("/v1/permissions/check", json=check).json()["allowed"] is False

        expiring_response = client.post(
            "/v1/permissions/grants",
            json={
                "subject": "alfred",
                "scope_kind": "workspace",
                "scope_id": "ws-expiring",
                "action_class": "openspec:write",
                "max_criticality": "low",
                "justification": "temporary grant",
                "requester": "alice",
                "approver": "owner-1",
            },
        )
        expiring = store.grants[expiring_response.json()["id"]]

        store.grants[expiring.id] = expiring.model_copy(update={"expires_at": datetime.utcnow() - timedelta(days=1)})
        expired = client.post("/v1/permissions/expire")
        assert expired.status_code == 200
        assert expired.json()["expired"] == 1
