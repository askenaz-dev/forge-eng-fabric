from __future__ import annotations

from forge_mcp import MCPServer, ToolRequest

server = MCPServer(name="confluence")


@server.tool("mcp:confluence.page.read")
async def read_page(request: ToolRequest) -> dict[str, object]:
    return {"page_id": request.params.get("page_id"), "title": "", "body": ""}


@server.tool("mcp:confluence.page.write")
async def write_page(request: ToolRequest) -> dict[str, object]:
    return {"page_id": request.params.get("page_id") or "local-page", "updated": True}


app = server.app
