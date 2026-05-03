from __future__ import annotations

import uuid
from datetime import datetime, timedelta

from fastapi.testclient import TestClient

from approvals.app import create_app
from approvals.events import InMemoryEventPublisher
from approvals.store import InMemoryApprovalStore


def test_create_list_decide_and_expire_approvals() -> None:
    store = InMemoryApprovalStore()
    publisher = InMemoryEventPublisher()
    app = create_app(store=store, publisher=publisher)
    workspace_id = str(uuid.uuid4())

    with TestClient(app) as client:
        created = client.post(
            "/v1/approvals",
            json={
                "principal": "alfred",
                "action": "deploy:prod",
                "workspace_id": workspace_id,
                "target": {"env": "prod"},
                "rationale": "production deploy",
                "required_approvers": ["release-manager"],
                "criticality": "high",
                "correlation_id": "corr-approval",
                "expiration_minutes": 30,
            },
        )
        assert created.status_code == 201
        approval_id = created.json()["id"]

        inbox = client.get("/v1/approvals", params={"approver": "release-manager", "status": "pending"})
        assert inbox.status_code == 200
        assert [item["id"] for item in inbox.json()["approvals"]] == [approval_id]

        decided = client.post(
            f"/v1/approvals/{approval_id}/decisions",
            json={"actor": "release-manager", "decision": "approved", "comment": "ship it"},
        )
        assert decided.status_code == 200
        assert decided.json()["status"] == "approved"

        expiring_response = client.post(
            "/v1/approvals",
            json={
                "principal": "alfred",
                "action": "deploy:stage",
                "workspace_id": workspace_id,
                "target": {"env": "stage"},
                "rationale": "stale approval",
                "required_approvers": ["release-manager"],
                "criticality": "medium",
                "correlation_id": "corr-expire",
                "expiration_minutes": 1,
            },
        )
        expiring_id = expiring_response.json()["id"]
        expiring = store.approvals[expiring_id]
        store.approvals[expiring.id] = expiring.model_copy(update={"expires_at": datetime.utcnow() - timedelta(minutes=1)})

        expired = client.post("/v1/approvals/expire")
        assert expired.status_code == 200
        assert expired.json()["expired"] == 1

    assert [event["type"] for event in publisher.events][:3] == [
        "approval.requested.v1",
        "approval.notification.queued.v1",
        "approval.decided.v1",
    ]
