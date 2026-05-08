from __future__ import annotations

from fastapi.testclient import TestClient

from servers import confluence as confluence_module
from servers.confluence import ConfluenceGuardrails, app as confluence_app


def _invoke(client: TestClient, tool_id: str, params: dict, context: dict | None = None):
    body = {"tool_id": tool_id, "params": params}
    if context is not None:
        body["context"] = context
    return client.post("/v1/invoke", json=body)


def test_create_page_adds_header_label_and_event() -> None:
    confluence_module._event_bus.events.clear()
    with TestClient(confluence_app) as client:
        resp = _invoke(
            client,
            "mcp:confluence.create_page",
            {"space_key": "ENG", "title": "Design", "body": "Body", "openspec_id": "spec-7"},
            context={"workspace_id": "ws-1"},
        )
        assert resp.status_code == 200, resp.text
        body = resp.json()["result"]
        assert "forge-managed" in body["labels"]
        assert "OpenSpec:</strong> spec-7" in body["body"]
        assert any(event["type"] == "confluence.page.created.v1" for event in confluence_module._event_bus.events)


def test_workspace_space_mapping_denies_unmapped_space() -> None:
    original = confluence_module._guardrails
    confluence_module._guardrails = ConfluenceGuardrails(workspace_space_map={"ws-1": {"ENG"}})
    confluence_module._event_bus.events.clear()
    try:
        with TestClient(confluence_app) as client:
            resp = _invoke(
                client,
                "mcp:confluence.create_page",
                {"space_key": "OPS", "title": "Wrong space"},
                context={"workspace_id": "ws-1"},
            )
            assert resp.status_code == 403
            assert "space_not_mapped" in resp.json()["detail"]
            assert any(event["type"] == "guardrail.trip.v1" for event in confluence_module._event_bus.events)
    finally:
        confluence_module._guardrails = original


def test_update_attach_label_search_and_webhook() -> None:
    confluence_module._event_bus.events.clear()
    with TestClient(confluence_app) as client:
        created = _invoke(client, "mcp:confluence.create_page", {"space_key": "ENG", "title": "Runbook", "body": "Initial"})
        page_id = created.json()["result"]["page_id"]

        updated = _invoke(client, "mcp:confluence.update_page", {"page_id": page_id, "space_key": "ENG", "title": "Runbook v2", "body": "Updated"})
        assert updated.status_code == 200
        assert updated.json()["result"]["updated"] is True

        attached = _invoke(client, "mcp:confluence.attach_file", {"page_id": page_id, "space_key": "ENG", "filename": "evidence.txt", "content": "ok"})
        assert attached.status_code == 200

        labelled = _invoke(client, "mcp:confluence.add_label", {"page_id": page_id, "space_key": "ENG", "label": "sdlc"})
        assert labelled.status_code == 200
        assert "sdlc" in labelled.json()["result"]["labels"]

        searched = _invoke(client, "mcp:confluence.search", {"query": "Runbook"})
        assert searched.status_code == 200
        assert page_id in str(searched.json()["result"])

        webhook = client.post("/v1/webhooks/confluence", json={"event": "page_updated", "page": {"id": page_id, "title": "Runbook v2", "space": {"key": "ENG"}}})
        assert webhook.status_code == 200
        assert webhook.json()["emitted"] == "confluence.page.updated.v1"
