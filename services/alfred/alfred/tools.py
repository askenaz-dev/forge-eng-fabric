"""Tool dispatch for Alfred.

The Phase 1 loop treats MCPs, Skills and Prompt Templates behind a single
async tool handler. Concrete MCP/Skill services arrive later in the phase; this
router keeps Alfred wired to the same invoke contract from day one.

alfred-litellm-header-injection (G2): `prompt:*` tool calls dispatch to
``prompt-template-service`` (`POST /v1/render`), not the legacy
`prompt-registry/v1/invoke` endpoint. The only accepted shape is
``prompt:<template_id>:render``. Other shapes raise `InvalidPromptToolId`.
Network errors and non-2xx responses surface as `ToolUnavailable` so the
reasoning loop can decide whether to retry or escalate.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

import httpx


class InvalidPromptToolId(ValueError):
    """Raised when a `prompt:*` tool id does not match `prompt:<template_id>:render`."""


class ToolUnavailable(RuntimeError):
    """Raised when an upstream tool service is unreachable or returned a non-2xx."""

    def __init__(self, tool_id: str, reason: str) -> None:
        super().__init__(f"tool_unavailable: {tool_id} ({reason})")
        self.tool_id = tool_id
        self.reason = reason


@dataclass(frozen=True)
class ToolRouter:
    mcp_endpoints: dict[str, str] = field(default_factory=dict)
    skill_endpoint: str | None = None
    # alfred-litellm-header-injection (G2): canonical prompt service.
    prompt_template_service_url: str | None = None
    timeout: float = 15.0

    async def invoke(self, tool_id: str, params: dict[str, Any]) -> dict[str, Any]:
        prefix, name = _split_tool_id(tool_id)
        if prefix == "mcp":
            server = name.split(".", 1)[0]
            base_url = self.mcp_endpoints.get(server)
            if not base_url:
                raise ValueError(f"no MCP endpoint configured for {server!r}")
            return await self._post_invoke(base_url, tool_id, params)
        if prefix == "skill":
            if not self.skill_endpoint:
                raise ValueError("no Skill runner endpoint configured")
            return await self._post_invoke(self.skill_endpoint, tool_id, params)
        if prefix == "prompt":
            return await self._render_prompt(tool_id, name, params)
        raise ValueError(f"unsupported tool id {tool_id!r}")

    async def _render_prompt(
        self, tool_id: str, name: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        template_id, action = _split_prompt_action(tool_id, name)
        if action != "render":
            raise InvalidPromptToolId(
                f"prompt tool id {tool_id!r} unsupported; expected "
                f"`prompt:<template_id>:render`"
            )
        if not self.prompt_template_service_url:
            raise ValueError(
                "prompt-template-service URL not configured "
                "(set PROMPT_TEMPLATE_SERVICE_URL)"
            )
        base = self.prompt_template_service_url.rstrip("/")
        body = {"ref": template_id, "variables": params}
        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                response = await client.post(f"{base}/v1/render", json=body)
        except httpx.HTTPError as exc:
            raise ToolUnavailable(tool_id, f"network error: {exc}") from exc
        if response.status_code >= 500 or response.status_code in {502, 503, 504}:
            raise ToolUnavailable(tool_id, f"upstream {response.status_code}")
        if response.status_code >= 400:
            raise ToolUnavailable(tool_id, f"upstream {response.status_code}: {response.text[:200]}")
        data = response.json()
        if isinstance(data, dict):
            return data
        return {"result": data}

    async def _post_invoke(
        self, base_url: str, tool_id: str, params: dict[str, Any]
    ) -> dict[str, Any]:
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


def _split_prompt_action(tool_id: str, name: str) -> tuple[str, str]:
    """Split `prompt:<template_id>:<action>` into (template_id, action).

    The leading prefix is already stripped by `_split_tool_id`; `name` is
    `<template_id>:<action>`. Anything that does not match raises
    `InvalidPromptToolId` with the canonical shape in the message.
    """

    if ":" not in name:
        raise InvalidPromptToolId(
            f"prompt tool id {tool_id!r} unsupported; expected "
            f"`prompt:<template_id>:render`"
        )
    template_id, action = name.rsplit(":", 1)
    if not template_id or not action:
        raise InvalidPromptToolId(
            f"prompt tool id {tool_id!r} unsupported; expected "
            f"`prompt:<template_id>:render`"
        )
    return template_id, action
