"""Smoke tests for every skill endpoint."""

from __future__ import annotations

import os
import tempfile

import pytest
from fastapi.testclient import TestClient

from sdlc_architecture_skills.events import MemorySink
from sdlc_architecture_skills.app import create_app


@pytest.fixture()
def sink() -> MemorySink:
    return MemorySink()


@pytest.fixture()
def client(sink: MemorySink) -> TestClient:
    app = create_app(sink=sink)
    return TestClient(app)


# ---------------------------------------------------------------------------
# /healthz
# ---------------------------------------------------------------------------


def test_healthz(client: TestClient) -> None:
    resp = client.get("/healthz")
    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}


# ---------------------------------------------------------------------------
# propose-adr
# ---------------------------------------------------------------------------


def test_propose_adr(client: TestClient, sink: MemorySink, tmp_path) -> None:
    resp = client.post(
        "/v1/skills/propose-adr",
        json={
            "tenant_id": "t-1",
            "workspace_id": "w-1",
            "title": "Use PostgreSQL for persistence",
            "context": "We need a relational store with ACID guarantees.",
            "decision_drivers": ["consistency", "familiarity"],
            "options_considered": ["PostgreSQL", "MySQL", "CockroachDB"],
            "output_dir": str(tmp_path),
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "ok"
    assert body["adr_number"] == 1
    assert body["slug"] == "use-postgresql-for-persistence"
    assert os.path.isfile(body["adr_path"])
    assert body["event_type"] == "sdlc.adr.proposed.v1"
    assert sink.events, "expected a CloudEvent"
    assert sink.events[0]["type"] == "sdlc.adr.proposed.v1"


def test_propose_adr_increments_number(client: TestClient, tmp_path) -> None:
    payload = {
        "tenant_id": "t-1",
        "title": "First ADR",
        "context": "ctx",
        "output_dir": str(tmp_path),
    }
    r1 = client.post("/v1/skills/propose-adr", json=payload)
    payload["title"] = "Second ADR"
    r2 = client.post("/v1/skills/propose-adr", json=payload)
    assert r1.json()["adr_number"] == 1
    assert r2.json()["adr_number"] == 2


# ---------------------------------------------------------------------------
# evaluate-options
# ---------------------------------------------------------------------------


def test_evaluate_options(client: TestClient) -> None:
    resp = client.post(
        "/v1/skills/evaluate-options",
        json={
            "tenant_id": "t-1",
            "decision_context": "Choose a message broker.",
            "options": [
                {"name": "Kafka", "description": "High-throughput distributed log"},
                {"name": "RabbitMQ", "description": "AMQP broker"},
            ],
            "criteria": ["throughput", "operational simplicity"],
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "ok"
    ranked = body["ranked_options"]
    assert len(ranked) == 2
    # First option should have higher or equal score than second.
    assert ranked[0]["score"] >= ranked[1]["score"]
    for opt in ranked:
        assert "pros" in opt
        assert "cons" in opt
        assert "rationale" in opt


# ---------------------------------------------------------------------------
# check-openspec-alignment
# ---------------------------------------------------------------------------


def test_check_openspec_alignment(client: TestClient) -> None:
    resp = client.post(
        "/v1/skills/check-openspec-alignment",
        json={
            "tenant_id": "t-1",
            "spec_path": "openspec/specs/my-service/spec.md",
            "requirements": ["REQ-1", "REQ-2", "REQ-3", "REQ-4"],
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "ok"
    assert "aligned" in body
    assert isinstance(body["unaddressed_requirements"], list)


def test_check_openspec_alignment_fully_aligned(client: TestClient) -> None:
    resp = client.post(
        "/v1/skills/check-openspec-alignment",
        json={
            "tenant_id": "t-1",
            "spec_path": "openspec/specs/my-service/spec.md",
            "requirements": [],
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["aligned"] is True
    assert body["unaddressed_requirements"] == []


# ---------------------------------------------------------------------------
# generate-api-contract
# ---------------------------------------------------------------------------


def test_generate_api_contract(client: TestClient, tmp_path) -> None:
    resp = client.post(
        "/v1/skills/generate-api-contract",
        json={
            "tenant_id": "t-1",
            "service_name": "my-service",
            "endpoints": [
                {"method": "post", "path": "/v1/widgets", "summary": "Create a widget"},
                {"method": "get", "path": "/v1/widgets/{id}", "summary": "Get a widget"},
            ],
            "output_dir": str(tmp_path),
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "ok"
    assert body["spectral_lint_passed"] is True
    assert os.path.isfile(body["openapi_path"])
    content = open(body["openapi_path"]).read()
    assert "openapi:" in content
    assert "/v1/widgets" in content


# ---------------------------------------------------------------------------
# propose-data-model
# ---------------------------------------------------------------------------


def test_propose_data_model(client: TestClient, sink: MemorySink, tmp_path) -> None:
    resp = client.post(
        "/v1/skills/propose-data-model",
        json={
            "tenant_id": "t-1",
            "workspace_id": "w-1",
            "domain": "Order Management",
            "entities": ["Order", "LineItem", "Customer"],
            "relationships": ["Order has many LineItems", "Order belongs to Customer"],
            "output_dir": str(tmp_path),
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "ok"
    assert os.path.isfile(body["model_path"])
    assert body["event_type"] == "sdlc.data_model.proposed.v1"
    assert any(e["type"] == "sdlc.data_model.proposed.v1" for e in sink.events)


# ---------------------------------------------------------------------------
# lightweight-threat-model
# ---------------------------------------------------------------------------


def test_lightweight_threat_model(client: TestClient, sink: MemorySink, tmp_path) -> None:
    resp = client.post(
        "/v1/skills/lightweight-threat-model",
        json={
            "tenant_id": "t-1",
            "workspace_id": "w-1",
            "system_name": "Payment Gateway",
            "trust_boundaries": ["internet", "internal-network"],
            "data_flows": ["browser -> API gateway", "API gateway -> payment processor"],
            "output_dir": str(tmp_path),
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "ok"
    assert os.path.isfile(body["threat_model_path"])
    assert body["event_type"] == "sdlc.threat_model.completed.v1"
    findings = body["findings"]
    assert len(findings) == 6  # one per STRIDE category
    categories = {f["category"] for f in findings}
    assert "Spoofing" in categories
    assert "Elevation of Privilege" in categories
    assert any(e["type"] == "sdlc.threat_model.completed.v1" for e in sink.events)
