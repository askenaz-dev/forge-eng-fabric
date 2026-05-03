from __future__ import annotations

from forge_mcp import MCPServer, ToolRequest

server = MCPServer(name="jira")


@server.tool("mcp:jira.issue.read")
async def read_issue(request: ToolRequest) -> dict[str, object]:
    return {"key": request.params.get("key"), "fields": {}}


@server.tool("mcp:jira.issue.write")
async def write_issue(request: ToolRequest) -> dict[str, object]:
    return {"key": request.params.get("key") or "LOCAL-1", "updated": True}


@server.tool("mcp:jira.sprint.update")
async def update_sprint(request: ToolRequest) -> dict[str, object]:
    return {"sprint": request.params.get("sprint"), "status": request.params.get("status")}


app = server.app
