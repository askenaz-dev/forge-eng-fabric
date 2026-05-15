"""Tests for app-first-class-entity changes (section 5).

Covers:
- IntentDraft and OpenSpecDocument carry `app_id` when supplied (5.1)
- `intent.committed.v1` event payload includes `app_id` (5.3)
- `POST /v1/specs/{id}:reparent` updates app_id and emits `spec.reparented.v1` (5.4)
- Commit refuses with `missing_app_scope` when the flag is on and app_id is unset (5.1)
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
    yield TestClient(app), publisher, app


def _start_and_fill(test_client, workspace_id: str, app_id: str | None) -> str:
    body = {"workspace_id": workspace_id, "created_by": "alice"}
    if app_id:
        body["app_id"] = app_id
    r = test_client.post("/v1/intent/start", json=body)
    assert r.status_code == 201, r.text
    draft_id = r.json()["draft_id"]
    r = test_client.post(
        f"/v1/intent/{draft_id}/answer",
        json={
            "answer": "Allow HR team to manage employee onboarding",
            "actor": "alice",
            "field_updates": {
                "title": "HR Portal",
                "business_intent": "HR team self-serve",
                "requirements_functional": ["Manage employees"],
            },
        },
    )
    assert r.status_code == 200, r.text
    return draft_id


def test_intent_committed_includes_app_id(client):
    test_client, publisher, _ = client
    workspace_id = str(uuid.uuid4())
    app_id = str(uuid.uuid4())
    draft_id = _start_and_fill(test_client, workspace_id, app_id)
    r = test_client.post(f"/v1/intent/{draft_id}/commit", json={"actor": "alice"})
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["app_id"] == app_id
    # event payload includes app_id
    committed = [ev for ev in publisher.events if ev["type"] == "intent.committed.v1"]
    assert len(committed) == 1
    assert committed[0]["data"]["app_id"] == app_id


def test_commit_refused_without_app_id_when_flag_on(client):
    test_client, _, app = client
    app.state.require_app_scope = True
    workspace_id = str(uuid.uuid4())
    draft_id = _start_and_fill(test_client, workspace_id, app_id=None)
    r = test_client.post(f"/v1/intent/{draft_id}/commit", json={"actor": "alice"})
    assert r.status_code == 422, r.text
    assert "missing_app_scope" in r.json()["detail"]


def test_reparent_emits_spec_reparented_event(client):
    test_client, publisher, _ = client
    workspace_id = str(uuid.uuid4())
    source_app_id = str(uuid.uuid4())
    target_app_id = str(uuid.uuid4())
    draft_id = _start_and_fill(test_client, workspace_id, source_app_id)
    r = test_client.post(f"/v1/intent/{draft_id}/commit", json={"actor": "alice"})
    assert r.status_code == 200, r.text
    spec_id = r.json()["openspec_id"]
    r = test_client.post(
        f"/v1/specs/{spec_id}:reparent",
        json={
            "target_app_id": target_app_id,
            "reason": "consolidating product surface",
            "actor": "alice",
        },
    )
    assert r.status_code == 200, r.text
    assert r.json()["app_id"] == target_app_id
    reparent_events = [ev for ev in publisher.events if ev["type"] == "spec.reparented.v1"]
    assert len(reparent_events) == 1
    payload = reparent_events[0]["data"]
    assert payload["spec_id"] == spec_id
    assert payload["from_app_id"] == source_app_id
    assert payload["to_app_id"] == target_app_id
    assert payload["reason"] == "consolidating product surface"


def test_reparent_refused_when_editor_missing_on_target(client):
    test_client, _, app = client
    workspace_id = str(uuid.uuid4())
    source_app_id = str(uuid.uuid4())
    target_app_id = str(uuid.uuid4())
    draft_id = _start_and_fill(test_client, workspace_id, source_app_id)
    r = test_client.post(f"/v1/intent/{draft_id}/commit", json={"actor": "alice"})
    spec_id = r.json()["openspec_id"]

    class DenyTargetAuthorizer:
        def can_edit_app(self, actor: str, app_id: str) -> bool:
            return app_id != target_app_id

    app.state.app_authorizer = DenyTargetAuthorizer()
    r = test_client.post(
        f"/v1/specs/{spec_id}:reparent",
        json={"target_app_id": target_app_id, "reason": "x", "actor": "alice"},
    )
    assert r.status_code == 403
    assert "missing_app_editor_on_target" in r.json()["detail"]
