from __future__ import annotations

from fastapi.testclient import TestClient

from reference_skills.evals import run_eval_suite
from reference_skills.runner import create_app
from reference_skills.skills import create_user_stories, generate_test_cases, scaffold_service

OPENSPEC = {
    "openspec_id": "os-payments",
    "title": "Payments reliability",
    "requirements": {"functional": ["Retry failed payments", "Emit audit event"]},
}


def test_create_user_stories_is_idempotent_and_linked() -> None:
    first = create_user_stories(OPENSPEC)
    second = create_user_stories(OPENSPEC)
    assert first == second
    assert first["stories"][0]["links"] == {"openspec_id": "os-payments", "direction": "bidirectional"}


def test_scaffold_service_generates_template_files() -> None:
    scaffold = scaffold_service(name="payments-api", language="python")
    assert "pyproject.toml" in scaffold["files"]
    assert scaffold["template"] == "forge-minimal-service"


def test_generate_test_cases_and_eval_suites_pass() -> None:
    cases = generate_test_cases(OPENSPEC)
    assert [case["id"] for case in cases["test_cases"]] == ["TC-001", "TC-002"]
    assert run_eval_suite("create-user-stories", create_user_stories, OPENSPEC)["passed"] is True
    assert run_eval_suite("scaffold-service", scaffold_service, name="worker")["passed"] is True
    assert run_eval_suite("generate-test-cases", generate_test_cases, OPENSPEC)["passed"] is True


def test_skill_runner_invokes_reference_skills() -> None:
    client = TestClient(create_app())
    response = client.post(
        "/v1/invoke",
        json={"tool_id": "skill:create-user-stories", "params": {"openspec": OPENSPEC}},
    )

    assert response.status_code == 200
    body = response.json()
    assert body["tool_id"] == "skill:create-user-stories"
    assert body["ok"] is True
    assert body["result"]["stories"][0]["links"]["openspec_id"] == "os-payments"
