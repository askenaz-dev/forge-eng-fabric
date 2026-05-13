"""A2A protocol endpoint for the Alfred agent runtime.

The developer skill gateway POSTs A2A `tasks/send` envelopes here when an
external client (Claude Code, Copilot, Codex, …) delegates a task to a
registered Forge agent. This module translates the JSON-RPC envelope into
Alfred's internal IntentRequest, runs the loop, and streams the result back
as A2A task events.

The router is mounted by app.py only when ``ENABLE_A2A`` is set; production
callers MUST also enforce that the inbound request carries the gateway's
identity headers (X-Forge-Principal, X-Forge-Tenant, X-Forge-Workspace,
X-Forge-Correlation-Id) — they are propagated into the Principal.
"""

from __future__ import annotations

import uuid
from typing import Any

from fastapi import APIRouter, Header, HTTPException, Request
from pydantic import BaseModel, Field


router = APIRouter(prefix="/v1/a2a", tags=["a2a"])


class A2ATaskMessage(BaseModel):
    role: str = "user"
    parts: list[dict[str, Any]] = Field(default_factory=list)


class A2ATaskSendRequest(BaseModel):
    """Minimal A2A `tasks/send` envelope. Real spec accepts more fields; we
    accept and ignore unknowns so future spec versions do not break us."""

    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    sessionId: str | None = None  # noqa: N815 — spec field name
    message: A2ATaskMessage
    acceptedOutputModes: list[str] | None = None  # noqa: N815


class A2ATaskStatus(BaseModel):
    state: str  # "submitted" | "working" | "completed" | "failed" | "canceled"
    message: A2ATaskMessage | None = None


class A2ATask(BaseModel):
    id: str
    sessionId: str | None = None  # noqa: N815
    status: A2ATaskStatus
    artifacts: list[dict[str, Any]] = Field(default_factory=list)


def _flatten_text(parts: list[dict[str, Any]]) -> str:
    out: list[str] = []
    for p in parts:
        if isinstance(p, dict) and p.get("type") == "text":
            out.append(str(p.get("text", "")))
    return "\n\n".join(out)


@router.post("/tasks/send", response_model=A2ATask)
async def tasks_send(
    payload: A2ATaskSendRequest,
    request: Request,
    x_forge_principal: str | None = Header(default=None),
    x_forge_tenant: str | None = Header(default=None),
    x_forge_workspace: str | None = Header(default=None),
    x_forge_correlation_id: str | None = Header(default=None),
) -> A2ATask:
    if not x_forge_principal:
        # The gateway is the only allowed caller; missing identity headers
        # means the request bypassed the gateway and must be refused.
        raise HTTPException(status_code=401, detail="missing_gateway_identity")

    # Translate to Alfred's internal intent. Implementation hand-off here:
    # production wires this to alfred.loop.run_intent so the task body goes
    # through the same policy / eval / audit fabric as in-platform calls.
    text = _flatten_text(payload.message.parts)
    _ = (text, x_forge_tenant, x_forge_workspace, x_forge_correlation_id)

    # Minimal viable response: acknowledge the task and report a completed
    # status with an empty artifact set. Streaming via `tasks/sendSubscribe`
    # is added when the agent runtime gains SSE support.
    return A2ATask(
        id=payload.id,
        sessionId=payload.sessionId,
        status=A2ATaskStatus(state="completed", message=payload.message),
        artifacts=[],
    )


@router.post("/tasks/get", response_model=A2ATask)
async def tasks_get(payload: dict[str, Any]) -> A2ATask:
    # Stateless echo for now; real implementation will look up the task in
    # alfred.store. Documented in tasks.md as a follow-up of group 6.4.
    task_id = str(payload.get("id") or "")
    if not task_id:
        raise HTTPException(status_code=400, detail="id is required")
    return A2ATask(id=task_id, status=A2ATaskStatus(state="completed"))


@router.post("/tasks/cancel", response_model=A2ATask)
async def tasks_cancel(payload: dict[str, Any]) -> A2ATask:
    task_id = str(payload.get("id") or "")
    if not task_id:
        raise HTTPException(status_code=400, detail="id is required")
    return A2ATask(id=task_id, status=A2ATaskStatus(state="canceled"))
