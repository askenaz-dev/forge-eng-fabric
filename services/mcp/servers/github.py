from __future__ import annotations

from typing import Any

from forge_mcp import MCPServer, ToolRequest

from servers.github_guardrails import GuardrailEngine
from servers.github_tokens import TokenIssuer

# Module-level so tests can override / patch.
_token_issuer = TokenIssuer()
_guardrails = GuardrailEngine()


def _audit_emit(audit: dict[str, Any]) -> None:
    # Hook point: production wires this to the audit kafka producer.
    # Kept as a no-op in this build; the audit payload is also returned in
    # tool results via the SDK's `audit` envelope.
    return None


def policy_hook(request: ToolRequest) -> tuple[bool, str]:
    # Read tools always allowed; write tools must pass guardrails.
    if request.tool_id in {
        "mcp:github.repo_metadata",
        "mcp:github.list_prs",
        "mcp:github.read_code",
    }:
        return True, "read_only_allowed"
    decision = _guardrails.check(
        tool_id=request.tool_id,
        params=request.params,
        workspace_id=request.context.workspace_id,
        principal=request.context.principal,
        correlation_id=request.context.correlation_id,
        approved_overrides=set(request.params.get("_overrides") or []),
    )
    _audit_emit(decision.audit)
    return decision.allowed, decision.rationale


server = MCPServer(name="github", policy_hook=policy_hook)


@server.tool("mcp:github.repo_metadata")
async def repo_metadata(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/local")
    return {"repo": repo, "default_branch": "main", "private": True, "principal": request.context.principal}


@server.tool("mcp:github.list_prs")
async def list_prs(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/local")
    return {"repo": repo, "pull_requests": []}


@server.tool("mcp:github.read_code")
async def read_code(request: ToolRequest) -> dict[str, object]:
    return {"repo": request.params.get("repo"), "path": request.params.get("path"), "content": ""}


def _issue_token_for(request: ToolRequest, repo: str, perms: dict[str, str]) -> dict[str, Any]:
    token = _token_issuer.issue(
        repo=repo,
        permissions=perms,
        principal=request.context.principal,
        correlation_id=request.context.correlation_id,
    )
    _audit_emit({"event": "github.installation_token.issued", **token.audit})
    return {
        "token_repo": token.repo,
        "token_expires_in": token.expires_in(),
        "token_audit": token.audit,
    }


@server.tool("mcp:github.create_repo")
async def create_repo(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    token_meta = _issue_token_for(request, repo, {"administration": "write", "contents": "write"})
    return {
        "repo": repo,
        "url": f"https://github.com/{repo}",
        "default_branch": "main",
        **token_meta,
    }


@server.tool("mcp:github.create_branch")
async def create_branch(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    branch = str(request.params.get("branch") or "feature")
    token_meta = _issue_token_for(request, repo, {"contents": "write"})
    return {"repo": repo, "branch": branch, "created": True, **token_meta}


@server.tool("mcp:github.create_pr")
async def create_pr(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    title = str(request.params.get("title") or "chore: bootstrap")
    pr_number = 1
    token_meta = _issue_token_for(request, repo, {"pull_requests": "write", "contents": "write"})
    return {
        "repo": repo,
        "pr_number": pr_number,
        "url": f"https://github.com/{repo}/pull/{pr_number}",
        "title": title,
        **token_meta,
    }


@server.tool("mcp:github.set_branch_protection")
async def set_branch_protection(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    branch = str(request.params.get("branch") or "main")
    rules = request.params.get("rules") or {}
    token_meta = _issue_token_for(request, repo, {"administration": "write"})
    return {"repo": repo, "branch": branch, "applied": True, "rules": rules, **token_meta}


@server.tool("mcp:github.set_codeowners")
async def set_codeowners(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    content = str(request.params.get("content") or "* @team-a")
    token_meta = _issue_token_for(request, repo, {"contents": "write"})
    return {"repo": repo, "codeowners": content, **token_meta}


@server.tool("mcp:github.add_pr_template")
async def add_pr_template(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    template = str(request.params.get("template") or "# PR Template")
    token_meta = _issue_token_for(request, repo, {"contents": "write"})
    return {"repo": repo, "template_added": True, "template_bytes": len(template), **token_meta}


@server.tool("mcp:github.set_required_checks")
async def set_required_checks(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    checks = request.params.get("checks") or []
    token_meta = _issue_token_for(request, repo, {"administration": "write", "checks": "write"})
    return {"repo": repo, "required_checks": checks, **token_meta}


app = server.app
