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


app = server.app
