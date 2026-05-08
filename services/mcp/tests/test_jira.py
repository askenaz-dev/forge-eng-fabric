from __future__ import annotations

from fastapi.testclient import TestClient

from servers import jira as jira_module
from servers.jira import JiraGuardrails, app as jira_app


def _invoke(client: TestClient, tool_id: str, params: dict, context: dict | None = None):
    body = {"tool_id": tool_id, "params": params}
    if context is not None:
        body["context"] = context
    return client.post("/v1/invoke", json=body)


def test_create_issue_and_epic_emit_events_without_plaintext_credentials() -> None:
    jira_module._event_bus.events.clear()
    with TestClient(jira_app) as client:
        resp = _invoke(
            client,
            "mcp:jira.create_issue",
            {
                "project_key": "ENG",
                "summary": "Implement traceability",
                "openspec_id": "spec-7",
                "auth": {"type": "api_token", "email": "bot@example.com", "api_token": "secret-token"},
            },
            context={"workspace_id": "ws-1"},
        )
        assert resp.status_code == 200, resp.text
        body = resp.json()["result"]
        assert body["key"].startswith("ENG-")
        assert body["auth_type"] == "api_token"
        assert "secret-token" not in str(body)

        epic = _invoke(client, "mcp:jira.create_epic", {"project_key": "ENG", "summary": "Epic"})
        assert epic.status_code == 200
        assert epic.json()["result"]["issue_type"] == "Epic"
        assert any(event["type"] == "jira.issue.created.v1" for event in jira_module._event_bus.events)


def test_workspace_project_mapping_denies_unmapped_project() -> None:
    original = jira_module._guardrails
    jira_module._guardrails = JiraGuardrails(workspace_project_map={"ws-1": {"ENG", "PLAT"}})
    jira_module._event_bus.events.clear()
    try:
        with TestClient(jira_app) as client:
            resp = _invoke(
                client,
                "mcp:jira.create_issue",
                {"project_key": "OPS", "summary": "Wrong project"},
                context={"workspace_id": "ws-1"},
            )
            assert resp.status_code == 403
            assert "project_not_mapped" in resp.json()["detail"]
            assert any(event["type"] == "guardrail.trip.v1" for event in jira_module._event_bus.events)
    finally:
        jira_module._guardrails = original


def test_rate_limit_and_circuit_breaker_are_reported() -> None:
    with TestClient(jira_app) as client:
        limited = _invoke(
            client,
            "mcp:jira.update_issue",
            {"key": "ENG-404", "fields": {"summary": "x"}, "_simulate_status": 429, "_retry_after_seconds": 7},
        )
        assert limited.status_code == 200
        assert limited.json()["result"]["rate_limited"] is True
        assert limited.json()["result"]["backoff_seconds"] == 7


def test_webhook_emits_jira_issue_event() -> None:
    jira_module._event_bus.events.clear()
    with TestClient(jira_app) as client:
        resp = client.post(
            "/v1/webhooks/jira",
            json={"webhookEvent": "jira:issue_updated", "issue": {"key": "ENG-100", "fields": {"status": {"name": "Done"}}}},
        )
        assert resp.status_code == 200
        assert resp.json()["emitted"] == "jira.issue.updated.v1"
        assert jira_module._event_bus.events[-1]["data"]["status"] == "Done"


def test_reconciliation_reports_drift_and_reconciled_items() -> None:
    jira_module._issues["ENG-200"] = {"key": "ENG-200", "status": "Done"}
    jira_module._event_bus.events.clear()
    with TestClient(jira_app) as client:
        resp = client.post(
            "/v1/reconcile",
            json={
                "linked_issues": [
                    {"openspec_id": "spec-7", "jira_key": "ENG-200", "expected_status": "Done"},
                    {"openspec_id": "spec-8", "jira_key": "ENG-201", "expected_status": "Done"},
                ],
            },
        )
        assert resp.status_code == 200
        assert resp.json() == {"reconciled": 1, "drift": 1, "interval_seconds": 900}
        assert any(event["type"] == "jira.issue.reconciled.v1" for event in jira_module._event_bus.events)
        assert any(event["type"] == "jira.issue.drift_detected.v1" for event in jira_module._event_bus.events)
