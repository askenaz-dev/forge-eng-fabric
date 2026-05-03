from __future__ import annotations

from forge_mcp import MCPServer, ToolRequest

server = MCPServer(name="github")


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


@server.tool("mcp:github.create_repo")
async def create_repo(request: ToolRequest) -> dict[str, object]:
    # Minimal stub: return repo metadata that would be created
    repo = str(request.params.get("repo") or "forge/new-repo")
    return {"repo": repo, "url": f"https://github.com/{repo}", "default_branch": "main"}


@server.tool("mcp:github.create_branch")
async def create_branch(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    branch = str(request.params.get("branch") or "feature")
    return {"repo": repo, "branch": branch, "created": True}


@server.tool("mcp:github.create_pr")
async def create_pr(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    title = str(request.params.get("title") or "chore: bootstrap")
    pr_number = 1
    return {"repo": repo, "pr_number": pr_number, "url": f"https://github.com/{repo}/pull/{pr_number}", "title": title}


@server.tool("mcp:github.set_branch_protection")
async def set_branch_protection(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    branch = str(request.params.get("branch") or "main")
    rules = request.params.get("rules") or {}
    return {"repo": repo, "branch": branch, "applied": True, "rules": rules}


@server.tool("mcp:github.set_codeowners")
async def set_codeowners(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    content = str(request.params.get("content") or "* @team-a")
    return {"repo": repo, "codeowners": content}


@server.tool("mcp:github.add_pr_template")
async def add_pr_template(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    template = str(request.params.get("template") or "# PR Template")
    return {"repo": repo, "template_added": True}


@server.tool("mcp:github.set_required_checks")
async def set_required_checks(request: ToolRequest) -> dict[str, object]:
    repo = str(request.params.get("repo") or "forge/new-repo")
    checks = request.params.get("checks") or []
    return {"repo": repo, "required_checks": checks}


app = server.app
