"""Integration tests for GitHub MCP write-mode tools.

Exercises the policy hook, guardrails, and short-lived token issuer end-to-end
through the FastAPI app (no real GitHub API call). When `FORGE_GITHUB_TEST_ORG`
is set, the tests in `test_github_real_org.py` (gated, optional) target the
real org; this file uses the in-process stub backend.
"""

from __future__ import annotations

import time

from fastapi.testclient import TestClient

from servers import github as github_module
from servers.github import app as github_app
from servers.github_guardrails import GuardrailEngine
from servers.github_tokens import TokenIssuer


def _invoke(client: TestClient, tool_id: str, params: dict, context: dict | None = None) -> dict:
    body = {"tool_id": tool_id, "params": params}
    if context is not None:
        body["context"] = context
    return client.post("/v1/invoke", json=body)


def test_create_repo_returns_short_lived_token() -> None:
    with TestClient(github_app) as client:
        resp = _invoke(client, "mcp:github.create_repo", {"repo": "org-a/svc-foo"})
        assert resp.status_code == 200, resp.text
        body = resp.json()["result"]
        assert body["repo"] == "org-a/svc-foo"
        assert body["token_repo"] == "org-a/svc-foo"
        # TTL must be ≤ 600 (10 min) per spec
        assert 0 < body["token_expires_in"] <= 600
        assert body["token_audit"]["permissions"]["administration"] == "write"


def test_set_branch_protection_emits_token_audit() -> None:
    with TestClient(github_app) as client:
        resp = _invoke(
            client,
            "mcp:github.set_branch_protection",
            {
                "repo": "org-a/svc-foo",
                "branch": "main",
                "rules": {"require_pr_review": True, "min_reviewers": 2},
            },
        )
        assert resp.status_code == 200
        body = resp.json()["result"]
        assert body["applied"] is True
        assert body["rules"]["min_reviewers"] == 2
        assert body["token_audit"]["permissions"]["administration"] == "write"


def test_guardrails_reject_invalid_repo_name() -> None:
    with TestClient(github_app) as client:
        resp = _invoke(client, "mcp:github.create_repo", {"repo": "org-a/has spaces!"})
        assert resp.status_code == 403
        assert "invalid_repo_name" in resp.json()["detail"]


def test_guardrails_reject_cross_workspace_org() -> None:
    # Inject a workspace map: ws-1 may only touch org-a.
    original = github_module._guardrails
    github_module._guardrails = GuardrailEngine(workspace_org_map={"ws-1": {"org-a"}})
    try:
        with TestClient(github_app) as client:
            resp = _invoke(
                client,
                "mcp:github.create_repo",
                {"repo": "org-b/svc-bar"},
                context={"workspace_id": "ws-1"},
            )
            assert resp.status_code == 403
            assert "cross_workspace_denied" in resp.json()["detail"]
    finally:
        github_module._guardrails = original


def test_guardrails_reject_org_not_in_allowlist() -> None:
    original = github_module._guardrails
    github_module._guardrails = GuardrailEngine(org_allowlist={"org-only"})
    try:
        with TestClient(github_app) as client:
            resp = _invoke(client, "mcp:github.create_repo", {"repo": "org-other/svc-x"})
            assert resp.status_code == 403
            assert "org_not_allowlisted" in resp.json()["detail"]
    finally:
        github_module._guardrails = original


def test_token_issuer_clamps_ttl() -> None:
    issuer = TokenIssuer(max_ttl_seconds=600)
    token = issuer.issue(repo="org-a/svc", ttl_seconds=86400)
    assert token.expires_in() <= 600
    assert token.audit["ttl_seconds"] == 600


def test_token_issuer_rejects_bad_repo() -> None:
    issuer = TokenIssuer()
    try:
        issuer.issue(repo="not-a-repo")
    except ValueError as exc:
        assert "org/name" in str(exc)
    else:
        raise AssertionError("expected ValueError")


def test_token_audit_includes_correlation_id() -> None:
    issuer = TokenIssuer()
    token = issuer.issue(repo="org-a/svc", correlation_id="corr-xyz", principal="alice")
    assert token.audit["correlation_id"] == "corr-xyz"
    assert token.audit["principal"] == "alice"
    assert token.expires_at > time.time()
