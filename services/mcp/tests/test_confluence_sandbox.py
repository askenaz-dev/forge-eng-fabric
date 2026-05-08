from __future__ import annotations

import os
import time

import pytest
from fastapi.testclient import TestClient

from servers import confluence as confluence_module
from servers.confluence import ConfluenceClient, app as confluence_app


def _sandbox_config() -> dict[str, str]:
    keys = {
        "base_url": "FORGE_CONFLUENCE_SANDBOX_URL",
        "email": "FORGE_CONFLUENCE_SANDBOX_EMAIL",
        "token": "FORGE_CONFLUENCE_SANDBOX_TOKEN",
        "space_key": "FORGE_CONFLUENCE_SANDBOX_SPACE_KEY",
    }
    values = {name: os.getenv(env, "").strip() for name, env in keys.items()}
    missing = [env for name, env in keys.items() if not values[name]]
    if missing:
        pytest.skip(f"Confluence sandbox env vars missing: {', '.join(missing)}")
    return values


def _invoke(client: TestClient, tool_id: str, params: dict, context: dict | None = None):
    body = {"tool_id": tool_id, "params": params}
    if context:
        body["context"] = context
    return client.post("/v1/invoke", json=body)


@pytest.mark.e2e
def test_confluence_sandbox_create_update_label_and_search() -> None:
    cfg = _sandbox_config()
    original_client = confluence_module._confluence_client
    confluence_module._confluence_client = ConfluenceClient(backend="atlassian", base_url=cfg["base_url"])
    try:
        with TestClient(confluence_app) as client:
            auth = {"type": "api_token", "email": cfg["email"], "api_token": cfg["token"]}
            title = f"Forge Confluence MCP E2E {int(time.time())}"
            created = _invoke(
                client,
                "mcp:confluence.create_page",
                {
                    "space_key": cfg["space_key"],
                    "title": title,
                    "body": "<p>Created by Forge MCP sandbox E2E test.</p>",
                    "openspec_id": "phase-4-sdlc-orchestration",
                    "auth": auth,
                },
                context={"workspace_id": "sandbox", "correlation_id": "confluence-sandbox-e2e"},
            )
            assert created.status_code == 200, created.text
            page_id = created.json()["result"]["page_id"]

            updated = _invoke(
                client,
                "mcp:confluence.update_page",
                {
                    "page_id": page_id,
                    "space_key": cfg["space_key"],
                    "title": f"{title} updated",
                    "body": "<p>Updated by Forge MCP sandbox E2E test.</p>",
                    "openspec_id": "phase-4-sdlc-orchestration",
                    "auth": auth,
                },
            )
            assert updated.status_code == 200, updated.text

            labelled = _invoke(
                client,
                "mcp:confluence.add_label",
                {"page_id": page_id, "space_key": cfg["space_key"], "label": "forge-managed", "auth": auth},
            )
            assert labelled.status_code == 200, labelled.text

            searched = _invoke(
                client,
                "mcp:confluence.search",
                {"cql": f'title ~ "{title}"', "auth": auth},
            )
            assert searched.status_code == 200, searched.text
            assert page_id in str(searched.json()["result"])
    finally:
        confluence_module._confluence_client = original_client
