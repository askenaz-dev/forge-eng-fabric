from __future__ import annotations

import os
import time

import pytest
from fastapi.testclient import TestClient

from servers import jira as jira_module
from servers.jira import RateLimitAwareJiraClient, app as jira_app


def _sandbox_config() -> dict[str, str]:
    keys = {
        "base_url": "FORGE_JIRA_SANDBOX_URL",
        "email": "FORGE_JIRA_SANDBOX_EMAIL",
        "token": "FORGE_JIRA_SANDBOX_TOKEN",
        "project_key": "FORGE_JIRA_SANDBOX_PROJECT_KEY",
    }
    values = {name: os.getenv(env, "").strip() for name, env in keys.items()}
    missing = [env for name, env in keys.items() if not values[name]]
    if missing:
        pytest.skip(f"Jira sandbox env vars missing: {', '.join(missing)}")
    return values


def _invoke(client: TestClient, tool_id: str, params: dict, context: dict | None = None):
    body = {"tool_id": tool_id, "params": params}
    if context:
        body["context"] = context
    return client.post("/v1/invoke", json=body)


@pytest.mark.e2e
def test_jira_sandbox_create_update_comment_and_search() -> None:
    cfg = _sandbox_config()
    original_client = jira_module._jira_client
    jira_module._jira_client = RateLimitAwareJiraClient(backend="atlassian", base_url=cfg["base_url"])
    try:
        with TestClient(jira_app) as client:
            auth = {"type": "api_token", "email": cfg["email"], "api_token": cfg["token"]}
            summary = f"Forge Jira MCP E2E {int(time.time())}"
            created = _invoke(
                client,
                "mcp:jira.create_issue",
                {
                    "project_key": cfg["project_key"],
                    "summary": summary,
                    "description": "Created by Forge MCP sandbox E2E test.",
                    "issue_type": os.getenv("FORGE_JIRA_SANDBOX_ISSUE_TYPE", "Task"),
                    "openspec_id": "phase-4-sdlc-orchestration",
                    "auth": auth,
                },
                context={"workspace_id": "sandbox", "correlation_id": "jira-sandbox-e2e"},
            )
            assert created.status_code == 200, created.text
            key = created.json()["result"]["key"]

            updated = _invoke(
                client,
                "mcp:jira.update_issue",
                {"key": key, "fields": {"summary": f"{summary} updated"}, "auth": auth},
            )
            assert updated.status_code == 200, updated.text

            commented = _invoke(
                client,
                "mcp:jira.add_comment",
                {"key": key, "comment": "Forge MCP sandbox E2E comment.", "auth": auth},
            )
            assert commented.status_code == 200, commented.text

            searched = _invoke(client, "mcp:jira.search", {"jql": f"key = {key}", "auth": auth})
            assert searched.status_code == 200, searched.text
            assert key in str(searched.json()["result"])
    finally:
        jira_module._jira_client = original_client
