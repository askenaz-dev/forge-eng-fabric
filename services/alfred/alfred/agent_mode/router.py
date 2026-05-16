"""HTTP routes for Alfred agent-mode sessions.

Mounted under ``/v1/agent-mode`` only when ``ALFRED_AGENT_MODE_ENABLED=true``.
Each route is gated by the same FGA workspace check used by the existing
``/v1/intents`` route.
"""

from __future__ import annotations

import asyncio
import json
import time
import uuid
from datetime import datetime
from typing import Annotated, Any, AsyncIterator

from fastapi import APIRouter, Depends, Header, HTTPException, Request, Response
from fastapi.responses import StreamingResponse

from alfred.agent_mode.events import EventEmitter
from alfred.agent_mode.executor import ExecutorDeps, cancel, execute_session, resume
from alfred.agent_mode.models import (
    ALLOWED_START_STEPS,
    COMMITTED_LIFECYCLE_STATES,
    AgentModeSession,
    FollowUpRequest,
    StartSessionRequest,
)
from alfred.agent_mode.planner import build_initial_plan
from alfred.agent_mode.store import AgentModeStore
from alfred.auth import Principal, fga_check, mint_sub_principal
from alfred.autonomy_presets import PresetStore
from alfred.config import Settings
from alfred.llm import RequestContext
from alfred.logging import get_logger

log = get_logger(__name__)

SSE_HEARTBEAT_SECONDS = 15.0


def build_router(
    *,
    settings: Settings,
    agent_store: AgentModeStore,
    executor_deps: ExecutorDeps,
    preset_store: PresetStore,
    event_emitter: EventEmitter,
    principal_dep,
) -> APIRouter:
    """Build the agent-mode APIRouter; mount only when the feature is enabled."""

    router = APIRouter(prefix="/v1/agent-mode", tags=["alfred-agent-mode"])

    async def _require_workspace(
        request: Request, principal: Principal, workspace_id: uuid.UUID, relation: str
    ) -> None:
        if not request.app.state.auth_required:
            return
        ok = await fga_check(
            base_url=settings.openfga_url,
            store_id=settings.openfga_store,
            model_id=settings.openfga_model,
            user=f"user:{principal.sub}",
            relation=relation,
            object_=f"workspace:{workspace_id}",
        )
        if not ok:
            raise HTTPException(status_code=403, detail=f"workspace {relation} required")

    @router.post("/sessions")
    async def start_session(
        request: Request,
        body: StartSessionRequest,
        response: Response,
        principal: Annotated[Principal, Depends(principal_dep)],
    ) -> dict[str, Any]:
        await _require_workspace(
            request, principal, body.workspace_id, "can_alfred_agent_mode_run"
        )
        # Non-human trigger enforcement (D3):
        # trigger_source != "human" MUST have actor="system:alfred" and a symptom_id.
        if body.trigger_source != "human":
            if body.actor != "system:alfred":
                raise HTTPException(
                    status_code=403,
                    detail="non-human trigger_source requires actor=system:alfred",
                )
            if not body.symptom_id:
                raise HTTPException(
                    status_code=422,
                    detail="non-human trigger_source requires symptom_id",
                )
            # Callers other than the triager are forbidden from using trigger_source=symptom.
            # The triager identifies itself via the X-Forge-Trigger-Source header (checked
            # by the auth middleware in a production deployment).

        # Workspace flag (D8).
        ws_settings = preset_store.get_settings(body.workspace_id)
        if not ws_settings.get("dock_enabled", False) and body.trigger_source == "human":
            raise HTTPException(
                status_code=409,
                detail="agent-mode is not enabled for this workspace",
            )

        # alfred-console-redesign 7.1: validate start_step.
        start_step = body.start_step
        if start_step is not None and start_step not in ALLOWED_START_STEPS:
            raise HTTPException(
                status_code=422,
                detail=f"unknown_start_step — allowed values: {sorted(ALLOWED_START_STEPS)}",
            )

        # alfred-console-redesign 7.2: enforce spec lifecycle gate.
        if start_step == "architect":
            spec = await executor_deps.openspec.get(body.openspec_id)
            lifecycle = (spec or {}).get("lifecycle_state", "proposed")
            if lifecycle not in COMMITTED_LIFECYCLE_STATES:
                raise HTTPException(
                    status_code=409,
                    detail=f"spec_not_ready_for_architect — lifecycle_state={lifecycle}",
                )

        correlation_id = body.correlation_id or request.state.correlation_id
        response.headers["X-Correlation-Id"] = correlation_id

        # Snapshot the active preset at start (D4).
        presets = preset_store.get_or_create(body.workspace_id)
        active_preset = next(iter(presets), {})

        # alfred-console-redesign 7.3: build plan starting at start_step.
        # alfred-litellm-header-injection (G1): planner LLM call carries the
        # four standard headers via RequestContext built from the session's
        # workspace + correlation. tenant_id derived from JWT claims when
        # present; falls back to dev sentinel only outside auth-required mode.
        tenant_claim = (
            principal.raw.get("forge_tenant_id")
            or principal.raw.get("tenant_id")
            or ("dev-tenant" if not request.app.state.auth_required else "")
        )
        llm_context = RequestContext(
            tenant_id=tenant_claim or "",
            workspace_id=str(body.workspace_id),
            correlation_id=correlation_id,
        )
        plan = await build_initial_plan(
            workspace_id=body.workspace_id,
            openspec_id=body.openspec_id,
            intent=body.intent,
            correlation_id=correlation_id,
            llm=executor_deps.llm,
            rag=executor_deps.rag,
            openspec=executor_deps.openspec,
            model=settings.agent_mode_default_model,
            start_step=start_step,
            llm_context=llm_context,
        )

        session = AgentModeSession(
            workspace_id=body.workspace_id,
            openspec_id=body.openspec_id,
            correlation_id=correlation_id,
            originator_principal=principal.sub,
            model_id=settings.agent_mode_default_model,
            plan_revision=1,
            plan_json=plan,
            frozen_autonomy_policy=active_preset,
            status="planning",
            # Non-human trigger fields.
            trigger_source=body.trigger_source,
            actor=body.actor or principal.sub,
            actor_session=body.actor_session,
            symptom_id=body.symptom_id,
            playbook_id=body.playbook_id,
            parent_session_id=body.parent_session_id,
        )
        await agent_store.create_session(session)

        # Mint a scoped sub-principal for autonomous sessions so every tool call
        # in the session runs under system:alfred:session:<id> rather than the
        # full system:alfred principal.  Auto-revocation is scheduled in the
        # executor when the session reaches a terminal status.
        if body.actor == "system:alfred":
            asyncio.create_task(
                mint_sub_principal(
                    base_url=settings.openfga_url,
                    store_id=settings.openfga_store,
                    model_id=settings.openfga_model,
                    session_id=str(session.id),
                    workspace_id=str(body.workspace_id),
                )
            )

        # alfred-console-redesign 7.4: emit session_started event with start_step.
        await event_emitter.emit(
            "alfred.agent_mode.session_started.v1",
            {
                "session_id": str(session.id),
                "workspace_id": str(body.workspace_id),
                "openspec_id": body.openspec_id,
                "originator": principal.sub,
                "start_step": start_step or "discovery",
                "correlation_id": correlation_id,
            },
        )

        # Kick off execution. Fire-and-forget — the dock will subscribe via SSE.
        asyncio.create_task(_safe_execute(executor_deps, session))

        return {
            "session_id": str(session.id),
            "status": session.status,
            "correlation_id": correlation_id,
            "plan_revision": session.plan_revision,
            "start_step": start_step or "discovery",
        }

    @router.get("/sessions/{session_id}")
    async def get_state(
        request: Request,
        session_id: uuid.UUID,
        principal: Annotated[Principal, Depends(principal_dep)],
    ) -> dict[str, Any]:
        session = await agent_store.get_session(session_id)
        if not session:
            raise HTTPException(status_code=404, detail="session not found")
        await _require_workspace(request, principal, session.workspace_id, "can_view")
        steps = await agent_store.list_steps(session_id)
        return {
            "session": session.model_dump(mode="json"),
            "steps": [s.model_dump(mode="json") for s in steps],
        }

    @router.get("/sessions/{session_id}/decisions")
    async def list_decisions(
        request: Request,
        session_id: uuid.UUID,
        principal: Annotated[Principal, Depends(principal_dep)],
    ) -> dict[str, Any]:
        session = await agent_store.get_session(session_id)
        if not session:
            raise HTTPException(status_code=404, detail="session not found")
        await _require_workspace(request, principal, session.workspace_id, "can_view")
        decisions = await executor_deps.store.list_decisions(session_id=session_id, limit=500)
        return {"decisions": [d.model_dump(mode="json") for d in decisions]}

    @router.get("/sessions/{session_id}/stream")
    async def stream(
        request: Request,
        session_id: uuid.UUID,
        principal: Annotated[Principal, Depends(principal_dep)],
        last_event_id: Annotated[str | None, Header(alias="Last-Event-ID")] = None,
    ) -> StreamingResponse:
        session = await agent_store.get_session(session_id)
        if not session:
            raise HTTPException(status_code=404, detail="session not found")
        await _require_workspace(request, principal, session.workspace_id, "can_view")

        async def event_gen() -> AsyncIterator[bytes]:
            # Replay durable state first (D5). Each step + decision becomes an event.
            replay_idx = 0
            try:
                start_idx = int(last_event_id) if last_event_id else -1
            except ValueError:
                start_idx = -1

            steps = await agent_store.list_steps(session_id)
            for step in steps:
                if replay_idx > start_idx:
                    yield _sse_event(
                        replay_idx,
                        "step",
                        {
                            "idx": step.idx,
                            "kind": step.kind,
                            "status": step.status,
                            "summary": step.summary,
                        },
                    )
                replay_idx += 1

            current = await agent_store.get_session(session_id)
            if current and replay_idx > start_idx:
                yield _sse_event(
                    replay_idx,
                    "session",
                    {"status": current.status, "plan_revision": current.plan_revision},
                )

            # Live phase: poll for new step rows. In production this would be a
            # NATS/Kafka subscriber on the alfred.agent_mode.* events.
            last_status: str | None = current.status if current else None
            last_step_count = len(steps)
            heartbeat_at = time.monotonic() + SSE_HEARTBEAT_SECONDS
            while True:
                if await request.is_disconnected():
                    return
                await asyncio.sleep(0.5)
                fresh = await agent_store.get_session(session_id)
                if not fresh:
                    return
                fresh_steps = await agent_store.list_steps(session_id)
                for step in fresh_steps[last_step_count:]:
                    yield _sse_event(
                        replay_idx,
                        "step",
                        {
                            "idx": step.idx,
                            "kind": step.kind,
                            "status": step.status,
                            "summary": step.summary,
                        },
                    )
                    replay_idx += 1
                last_step_count = len(fresh_steps)
                if fresh.status != last_status:
                    yield _sse_event(
                        replay_idx,
                        "session",
                        {"status": fresh.status, "plan_revision": fresh.plan_revision},
                    )
                    replay_idx += 1
                    last_status = fresh.status
                if fresh.status in ("completed", "aborted", "failed"):
                    return
                if time.monotonic() >= heartbeat_at:
                    yield b": heartbeat\n\n"
                    heartbeat_at = time.monotonic() + SSE_HEARTBEAT_SECONDS

        return StreamingResponse(
            event_gen(),
            media_type="text/event-stream",
            headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
        )

    @router.post("/sessions/{session_id}/messages")
    async def follow_up(
        request: Request,
        session_id: uuid.UUID,
        body: FollowUpRequest,
        principal: Annotated[Principal, Depends(principal_dep)],
    ) -> dict[str, Any]:
        session = await agent_store.get_session(session_id)
        if not session:
            raise HTTPException(status_code=404, detail="session not found")
        await _require_workspace(request, principal, session.workspace_id, "can_edit")

        # Re-evaluate the follow-up against the **frozen** preset (D4 + spec OQ2 mitigation).
        action_class = "alfred:agent-mode.run"
        ceilings = (session.frozen_autonomy_policy or {}).get("ceilings", {})
        ceiling = ceilings.get(action_class)
        if ceiling == "restricted":
            await event_emitter.emit(
                "alfred.agent_mode.failed.v1",
                {
                    "session_id": str(session.id),
                    "workspace_id": str(session.workspace_id),
                    "reason": "autonomy.override.rejected.v1",
                    "ceiling": ceiling,
                },
            )
            raise HTTPException(
                status_code=403,
                detail="follow-up rejected by frozen autonomy ceiling",
            )

        # Append the follow-up as a tool step at the end of the current plan.
        plan = dict(session.plan_json)
        steps = list(plan.get("steps") or [])
        steps.append(
            {
                "idx": len(steps),
                "kind": "tool",
                "tool_id": "skill:follow-up",
                "criticality": "low",
                "summary": body.intent,
                "params": {"intent": body.intent},
            }
        )
        plan["steps"] = steps
        await agent_store.update_session(session_id, plan_json=plan)

        if session.status in ("paused_for_approval", "paused_for_budget"):
            # Still paused — caller has to release the gate first.
            return {"session_id": str(session_id), "status": session.status, "appended_idx": len(steps) - 1}
        # Otherwise resume execution.
        refreshed = await agent_store.get_session(session_id) or session
        asyncio.create_task(_safe_execute(executor_deps, refreshed, from_idx=len(steps) - 1))
        return {"session_id": str(session_id), "status": "running", "appended_idx": len(steps) - 1}

    @router.post("/sessions/{session_id}/cancel")
    async def cancel_session(
        request: Request,
        session_id: uuid.UUID,
        principal: Annotated[Principal, Depends(principal_dep)],
        body: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        session = await agent_store.get_session(session_id)
        if not session:
            raise HTTPException(status_code=404, detail="session not found")
        await _require_workspace(
            request, principal, session.workspace_id, "can_alfred_agent_mode_cancel"
        )
        reason = (body or {}).get("reason") or "cancelled by user"
        updated = await cancel(executor_deps, session_id, reason)
        return {"session_id": str(session_id), "status": updated.status if updated else "unknown"}

    @router.post("/sessions/{session_id}/_resume")
    async def resume_session(
        request: Request,
        session_id: uuid.UUID,
        principal: Annotated[Principal, Depends(principal_dep)],
    ) -> dict[str, Any]:
        """Internal hook used by the approvals listener; not in the public OpenAPI."""
        session = await agent_store.get_session(session_id)
        if not session:
            raise HTTPException(status_code=404, detail="session not found")
        await _require_workspace(request, principal, session.workspace_id, "can_edit")
        updated = await resume(executor_deps, session_id)
        return {"session_id": str(session_id), "status": updated.status if updated else "unknown"}

    return router


def _sse_event(idx: int, event: str, data: dict[str, Any]) -> bytes:
    payload = json.dumps(data, default=str)
    return f"id: {idx}\nevent: {event}\ndata: {payload}\n\n".encode()


async def _safe_execute(deps: ExecutorDeps, session: AgentModeSession, *, from_idx: int = 0) -> None:
    try:
        await execute_session(deps, session, from_idx=from_idx)
    except Exception as exc:
        log.warning("agent_mode_execute_failed", session_id=str(session.id), error=str(exc))
        await deps.agent_store.update_session(
            session.id, status="failed", aborted_reason=str(exc),
            completed_at=datetime.utcnow(),
        )
