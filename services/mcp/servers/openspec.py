from __future__ import annotations

import os

import httpx

from forge_mcp import MCPServer, RemoteTransport, ToolRequest

server = MCPServer(
    name="openspec",
    remote_transport=RemoteTransport(http_path_template="/v1/invoke"),
)


def _base_url() -> str:
    return os.getenv("OPENSPEC_URL", "http://localhost:8083").rstrip("/")


@server.tool("mcp:openspec.get")
async def get_openspec(request: ToolRequest) -> dict[str, object]:
    async with httpx.AsyncClient(timeout=10.0) as client:
        response = await client.get(f"{_base_url()}/v1/openspecs/{request.params['openspec_id']}")
        response.raise_for_status()
        return response.json()


@server.tool("mcp:openspec.create")
async def create_openspec(request: ToolRequest) -> dict[str, object]:
    async with httpx.AsyncClient(timeout=10.0) as client:
        response = await client.post(f"{_base_url()}/v1/openspecs", json=request.params)
        response.raise_for_status()
        return response.json()


@server.tool("mcp:openspec.link")
async def link_openspec(request: ToolRequest) -> dict[str, object]:
    openspec_id = request.params["openspec_id"]
    body = {"actor": request.context.principal, "link": request.params["link"]}
    async with httpx.AsyncClient(timeout=10.0) as client:
        response = await client.post(f"{_base_url()}/v1/openspecs/{openspec_id}/links", json=body)
        response.raise_for_status()
        return response.json()


app = server.app
