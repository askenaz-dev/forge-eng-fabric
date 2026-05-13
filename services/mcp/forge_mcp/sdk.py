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
    tenant_id: str | None = None
    correlation_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    data_classification: str = "internal"
    trust_level: str = "T1"
    secrets: dict[str, str] = Field(default_factory=dict)


class RemoteTransport(BaseModel):
    """Declares how the MCP server is exposed remotely through the developer
    skill gateway. When omitted, the MCP is stdio-only and cannot be
    gateway-published."""

    http_path_template: str | None = None
    sse_path_template: str | None = None
    auth_modes: list[str] = Field(default_factory=lambda: ["pat", "oidc_bearer"])
    health_path: str = "/healthz"


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
        remote_transport: RemoteTransport | None = None,
    ) -> None:
        self.name = name
        self.version = version
        self.policy_hook = policy_hook or (lambda _request: (True, "allowed"))
        self.audit_hook = audit_hook or (lambda _request, _result: None)
        # remote_transport declares HTTP/SSE endpoints when the MCP is meant
        # to be gateway-publishable. Stdio-only servers leave this as None.
        self.remote_transport = remote_transport
        self._tools: dict[str, ToolHandler] = {}
        self.app = FastAPI(title=f"Forge MCP {name}", version=version)
        self._mount_routes()

    def tool(self, tool_id: str) -> Callable[[ToolHandler], ToolHandler]:
        def register(handler: ToolHandler) -> ToolHandler:
            self._tools[tool_id] = handler
            return handler

        return register

    def manifest(self) -> dict[str, Any]:
        out: dict[str, Any] = {
            "name": self.name,
            "version": self.version,
            "tools": sorted(self._tools),
            "identity_propagation": True,
            "secret_brokering": "context.secrets",
            "telemetry": "correlation_id",
            "audit": True,
            "policy_hooks": True,
        }
        if self.remote_transport is not None:
            out["remote_transport"] = {
                "http": {"path_template": self.remote_transport.http_path_template} if self.remote_transport.http_path_template else None,
                "sse":  {"path_template": self.remote_transport.sse_path_template}  if self.remote_transport.sse_path_template  else None,
                "auth_modes":  self.remote_transport.auth_modes,
                "health_path": self.remote_transport.health_path,
            }
        return out

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
            x_forge_tenant: str | None = Header(default=None),
            x_forge_workspace: str | None = Header(default=None),
            x_correlation_id: str | None = Header(default=None),
        ) -> ToolResult:
            # Gateway-forwarded identity headers are canonical. Conflicting
            # claims in the inbound payload are ignored; the override is
            # surfaced in audit so forensics can spot a malicious client.
            header_override = False
            if x_forge_principal:
                if request.context.principal and request.context.principal != x_forge_principal:
                    header_override = True
                request.context.principal = x_forge_principal
            if x_forge_tenant:
                request.context.tenant_id = x_forge_tenant
            if x_forge_workspace:
                if request.context.workspace_id and request.context.workspace_id != x_forge_workspace:
                    header_override = True
                request.context.workspace_id = x_forge_workspace
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
                    "tenant_id": request.context.tenant_id,
                    "workspace_id": request.context.workspace_id,
                    "correlation_id": request.context.correlation_id,
                    "policy": rationale,
                    "header_override": header_override,
                },
            )
            await _maybe_await(self.audit_hook(request, wrapped))
            return wrapped


async def _maybe_await(value):
    if inspect.isawaitable(value):
        return await value
    return value
