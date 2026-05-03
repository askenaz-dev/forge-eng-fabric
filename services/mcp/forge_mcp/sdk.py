from __future__ import annotations

import inspect
import uuid
from collections.abc import Awaitable, Callable
from typing import Any

from fastapi import FastAPI, Header, HTTPException
from pydantic import BaseModel, Field


class ToolContext(BaseModel):
    principal: str = "alfred"
    workspace_id: str | None = None
    correlation_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    data_classification: str = "internal"
    trust_level: str = "T1"
    secrets: dict[str, str] = Field(default_factory=dict)


class ToolRequest(BaseModel):
    tool_id: str
    params: dict[str, Any] = Field(default_factory=dict)
    context: ToolContext = Field(default_factory=ToolContext)


class ToolResult(BaseModel):
    tool_id: str
    ok: bool = True
    result: dict[str, Any] = Field(default_factory=dict)
    audit: dict[str, Any] = Field(default_factory=dict)


PolicyHook = Callable[[ToolRequest], Awaitable[tuple[bool, str]] | tuple[bool, str]]
AuditHook = Callable[[ToolRequest, ToolResult], Awaitable[None] | None]
ToolHandler = Callable[[ToolRequest], Awaitable[dict[str, Any]] | dict[str, Any]]


class MCPServer:
    def __init__(
        self,
        *,
        name: str,
        version: str = "0.1.0",
        policy_hook: PolicyHook | None = None,
        audit_hook: AuditHook | None = None,
    ) -> None:
        self.name = name
        self.version = version
        self.policy_hook = policy_hook or (lambda _request: (True, "allowed"))
        self.audit_hook = audit_hook or (lambda _request, _result: None)
        self._tools: dict[str, ToolHandler] = {}
        self.app = FastAPI(title=f"Forge MCP {name}", version=version)
        self._mount_routes()

    def tool(self, tool_id: str) -> Callable[[ToolHandler], ToolHandler]:
        def register(handler: ToolHandler) -> ToolHandler:
            self._tools[tool_id] = handler
            return handler

        return register

    def manifest(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "version": self.version,
            "tools": sorted(self._tools),
            "identity_propagation": True,
            "secret_brokering": "context.secrets",
            "telemetry": "correlation_id",
            "audit": True,
            "policy_hooks": True,
        }

    def _mount_routes(self) -> None:
        @self.app.get("/healthz")
        async def healthz() -> dict[str, str]:
            return {"status": "ok"}

        @self.app.get("/v1/manifest")
        async def manifest() -> dict[str, Any]:
            return self.manifest()

        @self.app.post("/v1/invoke", response_model=ToolResult)
        async def invoke(
            request: ToolRequest,
            x_forge_principal: str | None = Header(default=None),
            x_correlation_id: str | None = Header(default=None),
        ) -> ToolResult:
            if x_forge_principal:
                request.context.principal = x_forge_principal
            if x_correlation_id:
                request.context.correlation_id = x_correlation_id
            handler = self._tools.get(request.tool_id)
            if not handler:
                raise HTTPException(status_code=404, detail="unknown tool")
            allowed, rationale = await _maybe_await(self.policy_hook(request))
            if not allowed:
                raise HTTPException(status_code=403, detail=f"policy denied: {rationale}")
            result = await _maybe_await(handler(request))
            wrapped = ToolResult(
                tool_id=request.tool_id,
                result=result,
                audit={
                    "principal": request.context.principal,
                    "workspace_id": request.context.workspace_id,
                    "correlation_id": request.context.correlation_id,
                    "policy": rationale,
                },
            )
            await _maybe_await(self.audit_hook(request, wrapped))
            return wrapped


async def _maybe_await(value):
    if inspect.isawaitable(value):
        return await value
    return value
