"""End-to-end test for the wizard draft lifecycle.

Covers task 3.12 of platform-gaps-closure: a user with `workspace.member` role
completes a wizard session and the resulting OpenSpec passes validation.
"""

from __future__ import annotations

import uuid
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from openspec_service.app import Settings, create_app
from openspec_service.drafts import DraftStore
from openspec_service.events import InMemoryEventPublisher
from openspec_service.store import FilesystemOpenSpecStore


@pytest.fixture
def client(tmp_path: Path):
    settings = Settings(
        openspec_root=str(tmp_path / "records"),
        drafts_root=str(tmp_path / "drafts"),
    )
    publisher = InMemoryEventPublisher()
    store = FilesystemOpenSpecStore(Path(settings.openspec_root))
    drafts = DraftStore(root=Path(settings.drafts_root))
    app = create_app(settings=settings, store=store, drafts=drafts, publisher=publisher)
    yield TestClient(app), publisher


def test_wizard_completes_intent_to_commit_and_emits_audit_events(client):
    test_client, publisher = client
    workspace_id = str(uuid.uuid4())

    # Step 1: start a draft
    r = test_client.post(
        "/v1/intent/start",
        json={
            "workspace_id": workspace_id,
            "created_by": "alice@example.com",
            "title": "Loyalty rewards engine",
            "business_intent": "Track purchase history and issue tier-based discounts to retail customers.",
            "correlation_id": "corr-123",
        },
    )
    assert r.status_code == 201, r.text
    draft = r.json()
    draft_id = draft["draft_id"]
    assert draft["status"] == "drafting"

    # Step 2: completeness should be partial (intent done, others empty)
    r = test_client.get(f"/v1/openspecs/{draft_id}/completeness")
    assert r.status_code == 200
    sections = {s["name"]: s["status"] for s in r.json()["sections"]}
    assert sections["intent"] in {"partial", "complete"}
    assert sections["requirements"] == "empty"

    # Step 3: answer follow-up questions, populating fields incrementally
    test_client.post(
        f"/v1/intent/{draft_id}/answer",
        json={
            "answer": "Customer Support, Operations, Marketing",
            "actor": "alice@example.com",
            "field_updates": {
                "stakeholders": ["Customer Support", "Operations", "Marketing"],
                "success_metrics": ["redemption rate >= 30%", "tier-up rate per quarter"],
            },
        },
    )
    test_client.post(
        f"/v1/intent/{draft_id}/answer",
        json={
            "answer": "Track purchases and tiers, issue discounts, integrate with POS",
            "actor": "alice@example.com",
            "field_updates": {
                "requirements_functional": [
                    "Track customer purchase history",
                    "Issue tier-based discount codes",
                    "Integrate with POS system at checkout",
                ],
                "requirements_non_functional": ["P99 < 200ms at checkout", "PCI-DSS scope minimization"],
                "constraints": ["No PII in audit logs"],
            },
        },
    )

    # Step 4: commit the draft. The commit succeeds because business_intent and
    # at least one functional requirement are present.
    r = test_client.post(f"/v1/intent/{draft_id}/commit", json={"actor": "alice@example.com"})
    assert r.status_code == 200, r.text
    document = r.json()
    assert document["openspec_id"]
    assert document["business_intent"].startswith("Track purchase")
    assert len(document["requirements"]["functional"]) == 3
    assert document["lifecycle_status"] == "committed"

    # Step 5: audit trail
    audit_kinds = [event["type"] for event in publisher.events]
    assert "intent.dialogue.started.v1" in audit_kinds
    assert "intent.dialogue.turn.v1" in audit_kinds
    assert "intent.committed.v1" in audit_kinds


def test_commit_rejected_when_no_functional_requirements(client):
    test_client, _ = client
    workspace_id = str(uuid.uuid4())

    r = test_client.post(
        "/v1/intent/start",
        json={
            "workspace_id": workspace_id,
            "created_by": "alice@example.com",
            "title": "Sketch",
            "business_intent": "Just kicking tires",
        },
    )
    draft_id = r.json()["draft_id"]

    r = test_client.post(f"/v1/intent/{draft_id}/commit", json={"actor": "alice@example.com"})
    assert r.status_code == 400
    assert "functional requirement" in r.json()["detail"]


def test_expire_inactive_marks_old_drafts_abandoned(client):
    from datetime import datetime, timedelta

    test_client, publisher = client
    workspace_id = str(uuid.uuid4())

    r = test_client.post(
        "/v1/intent/start",
        json={
            "workspace_id": workspace_id,
            "created_by": "alice@example.com",
            "business_intent": "stale draft",
        },
    )
    draft_id = r.json()["draft_id"]

    # Forcibly age the draft past the 14-day TTL.
    state = test_client.app.state.drafts.drafts[draft_id]
    state.last_active_at = datetime.utcnow() - timedelta(days=15)

    r = test_client.post("/v1/intent/expire-inactive")
    assert r.status_code == 200
    body = r.json()
    assert body["abandoned_count"] == 1
    assert draft_id in body["abandoned_ids"]

    # Subsequent answer attempts MUST be rejected — draft is abandoned.
    r = test_client.post(
        f"/v1/intent/{draft_id}/answer",
        json={"answer": "too late", "actor": "alice@example.com"},
    )
    assert r.status_code == 409
