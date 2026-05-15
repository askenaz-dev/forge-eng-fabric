"""Smoke tests for sdlc-qa-skills."""

from __future__ import annotations

import os

import pytest
from fastapi.testclient import TestClient

from sdlc_qa_skills.app import create_app
from sdlc_qa_skills.events import MemorySink
from sdlc_qa_skills.models import (
    CIFailedPayload,
    GenerateE2ETestsRequest,
    GenerateTestPlanRequest,
    TriageTestFailuresRequest,
)
from sdlc_qa_skills.skills import QASkillRunner


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def make_runner(sink: MemorySink | None = None) -> QASkillRunner:
    return QASkillRunner(sink=sink or MemorySink())


def make_client(sink: MemorySink | None = None) -> TestClient:
    runner = make_runner(sink)
    app = create_app(runner=runner)
    return TestClient(app)


# ---------------------------------------------------------------------------
# /healthz
# ---------------------------------------------------------------------------


def test_healthz():
    client = make_client()
    r = client.get("/healthz")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


# ---------------------------------------------------------------------------
# generate-test-plan
# ---------------------------------------------------------------------------


def test_generate_test_plan_returns_path(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    sink = MemorySink()
    client = make_client(sink)

    r = client.post(
        "/v1/skills/generate-test-plan",
        json={"api_contract_path": "contracts/openapi/my-service.yaml", "tenant_id": "t-1"},
    )
    assert r.status_code == 200
    body = r.json()
    assert body["test_plan_path"] == "tests/plans/my-service.md"
    assert body["spec_slug"] == "my-service"
    assert body["event_id"]
    assert os.path.isfile(tmp_path / "tests" / "plans" / "my-service.md")


def test_generate_test_plan_emits_event(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    sink = MemorySink()
    client = make_client(sink)

    client.post(
        "/v1/skills/generate-test-plan",
        json={"api_contract_path": "contracts/openapi/other.yaml"},
    )
    assert len(sink.events) == 1
    ev = sink.events[0]
    assert ev["type"] == "sdlc.test_plan.proposed.v1"
    assert ev["source"] == "forge://service/sdlc-qa-skills"


# ---------------------------------------------------------------------------
# generate-e2e-tests
# ---------------------------------------------------------------------------


def test_generate_e2e_tests_returns_suite_path(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    client = make_client()

    r = client.post(
        "/v1/skills/generate-e2e-tests",
        json={"test_plan_path": "tests/plans/my-service.md"},
    )
    assert r.status_code == 200
    body = r.json()
    assert body["e2e_suite_path"] == "tests/e2e/my-service"
    assert body["file_count"] == 2
    assert os.path.isdir(tmp_path / "tests" / "e2e" / "my-service")


def test_generate_e2e_tests_writes_spec_file(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    client = make_client()

    client.post(
        "/v1/skills/generate-e2e-tests",
        json={"test_plan_path": "tests/plans/alpha.md"},
    )
    assert os.path.isfile(tmp_path / "tests" / "e2e" / "alpha" / "alpha.spec.ts")


# ---------------------------------------------------------------------------
# triage-test-failures
# ---------------------------------------------------------------------------


def test_triage_test_failures_returns_hypotheses():
    sink = MemorySink()
    client = make_client(sink)

    r = client.post(
        "/v1/skills/triage-test-failures",
        json={"ci_run_id": "run-42", "pr_url": "https://github.com/org/repo/pull/99"},
    )
    assert r.status_code == 200
    body = r.json()
    assert body["ci_run_id"] == "run-42"
    assert len(body["top_hypotheses"]) >= 1
    assert body["top_hypotheses"][0]["confidence"] > 0
    assert body["affected_files"]
    assert body["proposed_patch"]
    assert body["event_id"]


def test_triage_emits_event():
    sink = MemorySink()
    client = make_client(sink)

    client.post(
        "/v1/skills/triage-test-failures",
        json={"ci_run_id": "run-1", "pr_url": "https://github.com/org/repo/pull/1"},
    )
    assert len(sink.events) == 1
    assert sink.events[0]["type"] == "sdlc.test_failure.triaged.v1"


# ---------------------------------------------------------------------------
# ci-failed hook
# ---------------------------------------------------------------------------


def test_ci_failed_hook_returns_triaged(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    sink = MemorySink()
    client = make_client(sink)

    r = client.post(
        "/v1/hooks/ci-failed",
        json={
            "ci_run_id": "run-77",
            "pr_url": "https://github.com/org/repo/pull/77",
            "app_id": "my-app",
            "targets": {"qa": "off"},
        },
    )
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "triaged"
    assert body["triage_event_id"]
    assert body["fix_pr_url"] is None  # qa=off → no auto-PR


def test_ci_failed_hook_rate_limits(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    from sdlc_qa_skills import app as app_module

    # Reset state between tests
    app_module._pr_last_triaged.clear()

    sink = MemorySink()
    client = make_client(sink)
    payload = {
        "ci_run_id": "run-88",
        "pr_url": "https://github.com/org/repo/pull/88",
        "app_id": "my-app",
    }

    r1 = client.post("/v1/hooks/ci-failed", json=payload)
    assert r1.json()["status"] == "triaged"

    r2 = client.post("/v1/hooks/ci-failed", json=payload)
    assert r2.json()["status"] == "rate_limited"

    # Cleanup
    app_module._pr_last_triaged.clear()


def test_ci_failed_hook_opens_fix_pr_when_autonomous(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    from sdlc_qa_skills import app as app_module

    app_module._pr_last_triaged.clear()

    sink = MemorySink()
    client = make_client(sink)

    r = client.post(
        "/v1/hooks/ci-failed",
        json={
            "ci_run_id": "run-99",
            "pr_url": "https://github.com/org/repo/pull/99",
            "app_id": "my-app",
            "targets": {"qa": "autonomous"},
        },
    )
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "triaged"
    # StubLLM produces a non-empty patch with confidence < 0.95 → fix PR should open
    assert body["fix_pr_url"] is not None

    app_module._pr_last_triaged.clear()
