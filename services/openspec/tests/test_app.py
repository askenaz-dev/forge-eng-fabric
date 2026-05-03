from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from openspec_service.app import create_app
from openspec_service.events import InMemoryEventPublisher
from openspec_service.store import FilesystemOpenSpecStore


def test_openspec_crud_decisions_links_and_events(tmp_path) -> None:
    publisher = InMemoryEventPublisher()
    store = FilesystemOpenSpecStore(tmp_path)
    app = create_app(store=store, publisher=publisher)
    workspace_id = str(uuid.uuid4())

    with TestClient(app) as client:
        created = client.post(
            "/v1/openspecs",
            json={
                "openspec_id": "os-1",
                "workspace_id": workspace_id,
                "title": "Payments",
                "business_intent": "Reduce payment failures",
                "problem_statement": "Retries are inconsistent",
                "requirements": {"functional": ["Retry failed payments"]},
                "created_by": "alice",
            },
        )
        assert created.status_code == 201
        assert created.json()["version"] == 1

        invalid = client.post(
            "/v1/openspecs",
            json={
                "workspace_id": workspace_id,
                "title": "Invalid",
                "business_intent": "",
                "problem_statement": "missing requirements",
                "requirements": {"functional": []},
                "created_by": "alice",
            },
        )
        assert invalid.status_code == 422

        patched = client.patch(
            "/v1/openspecs/os-1",
            json={"updated_by": "bob", "success_metrics": ["failure rate < 1%"]},
        )
        assert patched.status_code == 200
        assert patched.json()["version"] == 2

        decision = client.post(
            "/v1/openspecs/os-1/decisions",
            json={
                "actor": "alfred",
                "decision": "Use queue retries",
                "rationale": "Existing runbook recommends retries",
                "correlation_id": "corr-1",
            },
        )
        assert decision.status_code == 200
        assert decision.json()["decision_log"][0]["correlation_id"] == "corr-1"

        linked = client.post(
            "/v1/openspecs/os-1/links",
            json={"actor": "alice", "link": {"kind": "jira", "ref": "PROJ-1"}},
        )
        assert linked.status_code == 200
        assert linked.json()["linked_artifacts"][0]["ref"] == "PROJ-1"

        listed = client.get("/v1/openspecs", params={"workspace_id": workspace_id})
        assert listed.status_code == 200
        assert [doc["openspec_id"] for doc in listed.json()["openspecs"]] == ["os-1"]

        versions = client.get("/v1/openspecs/os-1/versions")
        assert versions.status_code == 200
        assert versions.json()["versions"] == [1, 2, 3, 4]

        version_one = client.get("/v1/openspecs/os-1/versions/1")
        assert version_one.status_code == 200
        assert version_one.json()["version"] == 1

    assert [event["type"] for event in publisher.events] == [
        "openspec.created.v1",
        "openspec.updated.v1",
        "openspec.updated.v1",
        "openspec.linked.v1",
    ]
    assert (tmp_path / workspace_id / "os-1.json").exists()
    assert (tmp_path / ".versions" / "os-1" / "v4.json").exists()
