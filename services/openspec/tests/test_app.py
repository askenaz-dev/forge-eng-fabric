from __future__ import annotations

import uuid
from pathlib import Path

from fastapi.testclient import TestClient

from openspec_service.app import Settings, create_app
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
        assert linked.json()["linked_artifacts"][0]["namespace"] == "jira"

        jira_hook = client.post(
            "/v1/hooks/jira",
            json={"openspec_id": "os-1", "key": "PROJ-2", "url": "https://jira/PROJ-2", "status": "Done"},
        )
        assert jira_hook.status_code == 200
        assert jira_hook.json()["decision_log"][-1]["type"] == "jira_link"
        assert jira_hook.json()["linked_artifacts"][-1]["ref"] == "jira:PROJ-2"

        listed = client.get("/v1/openspecs", params={"workspace_id": workspace_id})
        assert listed.status_code == 200
        assert [doc["openspec_id"] for doc in listed.json()["openspecs"]] == ["os-1"]

        versions = client.get("/v1/openspecs/os-1/versions")
        assert versions.status_code == 200
        assert versions.json()["versions"] == [1, 2, 3, 4, 5, 6]

        version_one = client.get("/v1/openspecs/os-1/versions/1")
        assert version_one.status_code == 200
        assert version_one.json()["version"] == 1

    assert [event["type"] for event in publisher.events] == [
        "openspec.created.v1",
        "openspec.updated.v1",
        "openspec.updated.v1",
        "openspec.linked.v1",
        "openspec.updated.v1",
    ]
    assert (tmp_path / workspace_id / "os-1.json").exists()
    assert (tmp_path / ".versions" / "os-1" / "v6.json").exists()


def test_autonomous_loop_marker_and_review(tmp_path) -> None:
    publisher = InMemoryEventPublisher()
    store = FilesystemOpenSpecStore(tmp_path)
    app = create_app(store=store, publisher=publisher)
    workspace_id = str(uuid.uuid4())

    with TestClient(app) as client:
        created = client.post(
            "/v1/openspecs",
            json={
                "openspec_id": "os-evo-1",
                "workspace_id": workspace_id,
                "title": "Add cache fallback",
                "business_intent": "Reduce 5xx caused by stale cache",
                "problem_statement": "Cache returned stale data during incident inc-42",
                "requirements": {"functional": ["Add fallback to DB read on cache miss"]},
                "created_by": "evolution-service",
                "source": "autonomous-loop",
            },
        )
        assert created.status_code == 201, created.text
        assert created.json()["source"] == "autonomous-loop"
        assert created.json()["review_status"] == "pending"

        # Reject path.
        rejected = client.post(
            "/v1/openspecs/os-evo-1/review",
            json={"approved": False, "reviewer": "alice", "comment": "duplicate"},
        )
        assert rejected.status_code == 200
        assert rejected.json()["review_status"] == "rejected"
        assert rejected.json()["reviewed_by"] == "alice"

        # Cannot review again.
        twice = client.post(
            "/v1/openspecs/os-evo-1/review",
            json={"approved": True, "reviewer": "bob"},
        )
        assert twice.status_code == 400

        # Create another and approve it.
        client.post(
            "/v1/openspecs",
            json={
                "openspec_id": "os-evo-2",
                "workspace_id": workspace_id,
                "title": "Auto-rotate API keys",
                "business_intent": "Reduce blast radius of leaked tokens",
                "problem_statement": "Postmortem inc-43 found long-lived tokens",
                "requirements": {"functional": ["Rotate keys every 30 days"]},
                "created_by": "evolution-service",
                "source": "autonomous-loop",
            },
        )
        approved = client.post(
            "/v1/openspecs/os-evo-2/review",
            json={"approved": True, "reviewer": "alice"},
        )
        assert approved.status_code == 200
        assert approved.json()["review_status"] == "approved"

        # Stats endpoint reports counts and acceptance ratio (1 approved, 1 rejected → 0.5).
        stats = client.get("/v1/evolution/stats")
        assert stats.status_code == 200
        body = stats.json()
        assert body["total"] == 2
        assert body["approved"] == 1
        assert body["rejected"] == 1
        assert body["pending"] == 0
        assert body["acceptance_ratio"] == 0.5

    review_events = [e for e in publisher.events if e["type"] == "openspec.autonomous_loop.reviewed.v1"]
    assert len(review_events) == 2


def test_review_rejects_non_autonomous_doc(tmp_path) -> None:
    publisher = InMemoryEventPublisher()
    store = FilesystemOpenSpecStore(tmp_path)
    app = create_app(store=store, publisher=publisher)
    workspace_id = str(uuid.uuid4())
    with TestClient(app) as client:
        client.post(
            "/v1/openspecs",
            json={
                "openspec_id": "os-human",
                "workspace_id": workspace_id,
                "title": "Manual change",
                "business_intent": "Author by human",
                "problem_statement": "Plain change",
                "requirements": {"functional": ["Do something"]},
                "created_by": "alice",
            },
        )
        resp = client.post(
            "/v1/openspecs/os-human/review",
            json={"approved": True, "reviewer": "bob"},
        )
        assert resp.status_code == 400


def test_create_writes_openspec_backing_artifacts(tmp_path: Path) -> None:
    settings = Settings(
        openspec_root=str(tmp_path / "records"),
        drafts_root=str(tmp_path / "drafts"),
        openspec_artifacts_root=str(tmp_path / "openspec"),
    )
    client = TestClient(create_app(settings=settings))
    workspace_id = str(uuid.uuid4())

    created = client.post(
        "/v1/openspecs",
        json={
            "openspec_id": "payments-retry",
            "workspace_id": workspace_id,
            "title": "Payments Retry",
            "business_intent": "Reduce failed checkout payments",
            "problem_statement": "Transient processor errors fail too many checkouts",
            "requirements": {"functional": ["Retry transient payment failures"]},
            "created_by": "alice",
        },
    )

    assert created.status_code == 201, created.text
    artifacts = created.json()["openspec_artifacts"]
    assert artifacts["change_id"] == "payments-retry"
    assert artifacts["files"] == [".openspec.yaml", "README.md", "specs/payments-retry/spec.md"]
    assert (tmp_path / "openspec" / "changes" / "payments-retry" / ".openspec.yaml").exists()
    spec_delta = (tmp_path / "openspec" / "changes" / "payments-retry" / "specs" / "payments-retry" / "spec.md").read_text(
        encoding="utf-8"
    )
    assert "## ADDED Requirements" in spec_delta
    assert "Retry transient payment failures" in spec_delta
