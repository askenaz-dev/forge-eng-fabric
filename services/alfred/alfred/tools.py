"""Tool dispatch for Alfred.

The Phase 1 loop treats MCPs, Skills and Prompt Templates behind a single
async tool handler. Concrete MCP/Skill services arrive later in the phase; this
router keeps Alfred wired to the same invoke contract from day one.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

import httpx


@dataclass(frozen=True)
class ToolRouter:
    mcp_endpoints: dict[str, str] = field(default_factory=dict)
    skill_endpoint: str | None = None
    prompt_endpoint: str | None = None
    timeout: float = 15.0

    async def invoke(self, tool_id: str, params: dict[str, Any]) -> dict[str, Any]:
        prefix, name = _split_tool_id(tool_id)
        if prefix == "mcp":
            server = name.split(".", 1)[0]
            base_url = self.mcp_endpoints.get(server)
            if not base_url:
                raise ValueError(f"no MCP endpoint configured for {server!r}")
            return await self._post(base_url, tool_id, params)
        if prefix == "skill":
            if not self.skill_endpoint:
                raise ValueError("no Skill runner endpoint configured")
            return await self._post(self.skill_endpoint, tool_id, params)
        if prefix == "prompt":
            if not self.prompt_endpoint:
                raise ValueError("no Prompt Registry endpoint configured")
            return await self._post(self.prompt_endpoint, tool_id, params)
        raise ValueError(f"unsupported tool id {tool_id!r}")

    async def _post(self, base_url: str, tool_id: str, params: dict[str, Any]) -> dict[str, Any]:
        async with httpx.AsyncClient(timeout=self.timeout) as client:
            response = await client.post(
                f"{base_url.rstrip('/')}/v1/invoke",
                json={"tool_id": tool_id, "params": params},
            )
            response.raise_for_status()
            data = response.json()
            if isinstance(data, dict):
                return data
            return {"result": data}


def _split_tool_id(tool_id: str) -> tuple[str, str]:
    if ":" not in tool_id:
        raise ValueError("tool id must be namespaced as '<kind>:<name>'")
    prefix, name = tool_id.split(":", 1)
    if not prefix or not name:
        raise ValueError("tool id must be namespaced as '<kind>:<name>'")
    return prefix, name
