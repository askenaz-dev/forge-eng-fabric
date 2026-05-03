from __future__ import annotations

from fastapi.testclient import TestClient

from forge_mcp import MCPServer, ToolRequest
from servers.github import app as github_app


def test_sdk_invokes_tool_with_identity_policy_and_audit() -> None:
    audit = []

    def policy(request: ToolRequest):
        return request.context.trust_level == "T1", "T1 only"

    def audit_hook(request, result):
        audit.append((request.tool_id, result.audit["principal"]))

    server = MCPServer(name="test", policy_hook=policy, audit_hook=audit_hook)

    @server.tool("mcp:test.echo")
    def echo(request: ToolRequest):
        return {"params": request.params, "principal": request.context.principal}

    with TestClient(server.app) as client:
        response = client.post(
            "/v1/invoke",
            json={"tool_id": "mcp:test.echo", "params": {"x": 1}, "context": {"trust_level": "T1"}},
            headers={"X-Forge-Principal": "alice", "X-Correlation-Id": "corr-mcp"},
        )
        assert response.status_code == 200
        assert response.json()["result"]["principal"] == "alice"
        assert response.json()["audit"]["correlation_id"] == "corr-mcp"

        denied = client.post(
            "/v1/invoke",
            json={"tool_id": "mcp:test.echo", "context": {"trust_level": "T0"}},
        )
        assert denied.status_code == 403

    assert audit == [("mcp:test.echo", "alice")]


def test_github_mcp_manifest_and_repo_metadata() -> None:
    with TestClient(github_app) as client:
        manifest = client.get("/v1/manifest")
        assert manifest.status_code == 200
        assert "mcp:github.repo_metadata" in manifest.json()["tools"]

        metadata = client.post(
            "/v1/invoke",
            json={"tool_id": "mcp:github.repo_metadata", "params": {"repo": "org/repo"}},
        )
        assert metadata.status_code == 200
        assert metadata.json()["result"]["repo"] == "org/repo"
