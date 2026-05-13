"""Plan executor for Alfred agent-mode sessions.

Per design D3, every step dispatches through the *same*
`PermissionsClient.can → PolicyClient.evaluate → ApprovalsClient.request → tool_handler`
sequence the wizard uses in `alfred.loop.run_intent`. Agent-mode is just a loop
over a stored plan, with HITL pauses, replanning, budget probes and frozen
preset enforcement layered on top.
"""

from __future__ import annotations

import time
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import datetime
from typing import Any

from alfred.agent_mode.models import (
    AgentModeSession,
    AgentModeStep,
    PlanRevision,
    SessionStatus,
    StepKind,
)
from alfred.agent_mode.planner import replan
from alfred.agent_mode.store import AgentModeStore
from alfred.gateways import ApprovalsClient, OpenSpecClient, PermissionsClient, PolicyClient, RAGClient
from alfred.llm import LiteLLMClient
from alfred.logging import get_logger
from alfred.loop import _action_class_for
from alfred.models import DecisionRecord
from alfred.observability import AIObserver
from alfred.redaction import redact
from alfred.store import Store

log = get_logger(__name__)

ToolHandler = Callable[[str, dict[str, Any]], Awaitable[dict[str, Any]]]
WorkflowDispatcher = Callable[[str, dict[str, Any], str], Awaitable[dict[str, Any]]]
"""Dispatch a workflow run. Args: workflow_id, params, correlation_id. Returns
a dict with at least ``run_id`` and may carry interleaved step events."""

BudgetProbe = Callable[[uuid.UUID], Awaitable[dict[str, Any]]]
"""Probe LiteLLM tenant budget. Returns ``{"status": "ok"|"over_budget", ...}``."""

EventEmitter = Callable[[str, dict[str, Any]], Awaitable[None]]
"""Emit a CloudEvent with (event_type, payload)."""


@dataclass
class ExecutorDeps:
    store: Store
    agent_store: AgentModeStore
    llm: LiteLLMClient
    rag: RAGClient
    policy: PolicyClient
    approvals: ApprovalsClient
    permissions: PermissionsClient
    openspec: OpenSpecClient
    tool_handler: ToolHandler
    workflow_dispatcher: WorkflowDispatcher | None
    budget_probe: BudgetProbe | None
    emit_event: EventEmitter | None
    ai_observer: AIObserver | None = None


class StepOutcome(BaseException):
    """Marker class — never raised. Subclasses signal control flow."""


class PauseFor(StepOutcome):
    def __init__(self, kind: str, payload: dict[str, Any]) -> None:
        self.kind = kind
        self.payload = payload


class AbortSession(StepOutcome):
    def __init__(self, reason: str, payload: dict[str, Any]) -> None:
        self.reason = reason
        self.payload = payload


async def execute_session(
    deps: ExecutorDeps,
    session: AgentModeSession,
    *,
    from_idx: int = 0,
) -> AgentModeSession:
    """Drive the plan starting at ``from_idx``.

    Updates session state in-place via the agent_store. Returns the latest
    session row. Pauses (approval / budget) and aborts (policy deny / cancel)
    early-return with the appropriate status set.
    """

    await _emit(deps, session, "alfred.agent_mode.resumed.v1" if from_idx > 0 else "alfred.agent_mode.session_started.v1", {})
    await deps.agent_store.update_session(session.id, status="running")
    session = (await deps.agent_store.get_session(session.id)) or session

    steps_by_idx = {s.idx: s for s in await deps.agent_store.list_steps(session.id)}
    plan_steps = session.plan_json.get("steps") or []

    for raw in plan_steps:
        idx = int(raw.get("idx", 0))
        if idx < from_idx:
            continue

        step = steps_by_idx.get(idx) or AgentModeStep(
            session_id=session.id,
            idx=idx,
            kind=raw.get("kind", "tool"),
            tool_id=raw.get("tool_id"),
            workflow_id=raw.get("workflow_id"),
            agent_id=raw.get("agent_id"),
            criticality=raw.get("criticality", "low"),
            summary=raw.get("summary", ""),
            params=raw.get("params") or {},
        )

        if step.status in ("succeeded", "skipped", "cancelled"):
            continue

        step.status = "running"
        step.started_at = datetime.utcnow()
        await deps.agent_store.upsert_step(step)
        await _emit(deps, session, "alfred.agent_mode.step_started.v1", {"step_idx": step.idx, "kind": step.kind})

        try:
            await _dispatch_step(deps, session, step, raw)
        except PauseFor as pause:
            step.status = (
                "paused_for_approval" if pause.kind == "approval" else "paused_for_budget"
            )
            await deps.agent_store.upsert_step(step)
            await deps.agent_store.update_session(
                session.id,
                status="paused_for_approval" if pause.kind == "approval" else "paused_for_budget",
                paused_at=datetime.utcnow(),
            )
            event_type = (
                "alfred.agent_mode.paused_for_approval.v1"
                if pause.kind == "approval"
                else "alfred.agent_mode.paused_for_budget.v1"
            )
            await _emit(deps, session, event_type, {"step_idx": step.idx, **pause.payload})
            return (await deps.agent_store.get_session(session.id)) or session
        except AbortSession as abort:
            step.status = "cancelled"
            await deps.agent_store.upsert_step(step)
            await deps.agent_store.update_session(
                session.id, status="aborted", aborted_reason=abort.reason,
                completed_at=datetime.utcnow(),
            )
            await _emit(deps, session, "alfred.agent_mode.aborted.v1", {"reason": abort.reason, **abort.payload})
            return (await deps.agent_store.get_session(session.id)) or session
        except Exception as exc:
            # Recoverable: replan and continue
            step.status = "failed"
            step.completed_at = datetime.utcnow()
            step.outcome = {"error": str(exc)}
            await deps.agent_store.upsert_step(step)
            new_plan = replan(session.plan_json, idx, str(exc))
            session = (
                await deps.agent_store.update_session(
                    session.id,
                    plan_revision=session.plan_revision + 1,
                    plan_json=new_plan,
                )
            ) or session
            await deps.agent_store.append_revision(
                session.id,
                PlanRevision(
                    revision=session.plan_revision,
                    plan_json=new_plan,
                    reason=f"step {idx} failed: {exc}",
                    inserted_step_idx=idx + 1,
                ),
            )
            await _emit(
                deps, session, "alfred.agent_mode.plan_revised.v1",
                {"revision": session.plan_revision, "reason": str(exc)},
            )
            return await execute_session(deps, session, from_idx=idx + 1)

        step.status = "succeeded"
        step.completed_at = datetime.utcnow()
        await deps.agent_store.upsert_step(step)
        await _emit(
            deps, session, "alfred.agent_mode.step_completed.v1",
            {"step_idx": step.idx, "outcome": (step.outcome or {})},
        )

    # All steps done.
    await deps.agent_store.update_session(
        session.id, status="completed", completed_at=datetime.utcnow()
    )
    await _emit(deps, session, "alfred.agent_mode.completed.v1", {})
    return (await deps.agent_store.get_session(session.id)) or session


async def resume(deps: ExecutorDeps, session_id: uuid.UUID) -> AgentModeSession | None:
    """Resume a paused session — called after approval grants or budget refresh."""
    session = await deps.agent_store.get_session(session_id)
    if not session:
        return None
    if session.status not in ("paused_for_approval", "paused_for_budget"):
        return session
    steps = await deps.agent_store.list_steps(session_id)
    next_idx = 0
    for s in steps:
        if s.status in ("succeeded", "skipped", "cancelled"):
            next_idx = s.idx + 1
        elif s.status in ("paused_for_approval", "paused_for_budget"):
            next_idx = s.idx
            break
    await deps.agent_store.update_session(
        session_id, status="running", resumed_at=datetime.utcnow()
    )
    session = (await deps.agent_store.get_session(session_id)) or session
    return await execute_session(deps, session, from_idx=next_idx)


async def cancel(deps: ExecutorDeps, session_id: uuid.UUID, reason: str) -> AgentModeSession | None:
    """Transition a running session to aborted. Permission check happens at API layer."""
    session = await deps.agent_store.get_session(session_id)
    if not session:
        return None
    if session.status in ("completed", "aborted", "failed"):
        return session
    await deps.agent_store.update_session(
        session_id, status="aborted", aborted_reason=reason,
        completed_at=datetime.utcnow(),
    )
    session = (await deps.agent_store.get_session(session_id)) or session
    await _emit(deps, session, "alfred.agent_mode.aborted.v1", {"reason": reason})
    return session


# --- per-step dispatch -----------------------------------------------------


async def _dispatch_step(
    deps: ExecutorDeps,
    session: AgentModeSession,
    step: AgentModeStep,
    raw: dict[str, Any],
) -> None:
    """Run a single step through the unified gate stack."""

    kind: StepKind = step.kind  # type: ignore[assignment]

    if kind == "final":
        step.outcome = {"summary": raw.get("summary", "")}
        return

    # Budget probe before any LLM-bound dispatch.
    if deps.budget_probe is not None and kind in ("tool", "agent", "workflow"):
        budget = await deps.budget_probe(session.workspace_id)
        if budget.get("status") == "over_budget":
            raise PauseFor("budget", {"budget": budget})

    tool_id = step.tool_id or step.workflow_id or step.agent_id or ""
    action_class = _action_class_for(tool_id) if kind == "tool" else _kind_action_class(kind, tool_id)
    params = step.params

    # 1) Permission gate
    perm = await deps.permissions.can(
        subject="alfred",
        action_class=action_class,
        scope_kind="workspace",
        scope_id=str(session.workspace_id),
        criticality=step.criticality,
    )
    if not perm.get("allowed"):
        decision = await _record_decision(
            deps, session, step, tool_id, params,
            outcome="failed",
            policy_evaluated={"permissions": perm},
            outcome_detail={"reason": "no delegated permission", "perm": perm},
        )
        step.decision_id = decision.id
        raise AbortSession(reason="permission_denied", payload={"perm": perm})

    # 2) Policy gate — also honors frozen autonomy preset ceilings (D4).
    frozen_ceiling = (session.frozen_autonomy_policy or {}).get("ceilings", {}).get(action_class)
    policy = await deps.policy.evaluate(
        principal="alfred",
        action=tool_id,
        workspace_id=session.workspace_id,
        openspec_id=session.openspec_id,
        target=params,
        env=str(params.get("env", "dev")),
        criticality=step.criticality,
    )
    p_decision = policy.get("decision", "allow")
    # Tighten the policy decision if frozen ceiling demands it.
    if frozen_ceiling in ("requires_approval", "requires_dual_control") and p_decision == "allow":
        p_decision = "requires_approval"
        policy = {**policy, "decision": p_decision, "frozen_ceiling": frozen_ceiling}

    if p_decision == "deny":
        decision = await _record_decision(
            deps, session, step, tool_id, params,
            outcome="failed",
            policy_evaluated=policy,
            outcome_detail={"reason": "policy:deny"},
        )
        step.decision_id = decision.id
        raise AbortSession(reason="policy_denied", payload={"policy": policy})

    if p_decision == "requires_approval":
        req = await deps.approvals.request(
            principal="alfred",
            action=tool_id,
            workspace_id=session.workspace_id,
            openspec_id=session.openspec_id,
            target=params,
            rationale=step.summary or "Alfred agent-mode dispatched step",
            required_approvers=policy.get("required_approvers") or ["workspace:owner"],
            criticality=step.criticality,
            correlation_id=session.correlation_id,
        )
        decision = await _record_decision(
            deps, session, step, tool_id, params,
            outcome="approved" if req.get("status") == "approved" else "pending",
            policy_evaluated=policy,
            outcome_detail={"approval": req},
        )
        step.decision_id = decision.id
        if req.get("status") == "approved":
            # Pre-approved — fall through to execution.
            pass
        else:
            raise PauseFor("approval", {"approval": req})

    # 3) Execute
    started = time.perf_counter()
    try:
        if kind == "workflow":
            if deps.workflow_dispatcher is None:
                raise RuntimeError("workflow dispatcher not configured")
            result = await deps.workflow_dispatcher(
                step.workflow_id or "", params, session.correlation_id
            )
            run_id = result.get("run_id")
            if run_id:
                await deps.agent_store.update_session(session.id, workflow_run_id=str(run_id))
        else:
            result = await deps.tool_handler(tool_id, params)
        outcome = "succeeded"
        outcome_detail: dict[str, Any] = {"result_preview": str(result)[:500]}
        step.outcome = result if isinstance(result, dict) else {"value": result}
        error: str | None = None
    except (PauseFor, AbortSession):
        raise
    except Exception as exc:
        outcome = "failed"
        outcome_detail = {"error": str(exc)}
        error = str(exc)
        result = {"error": str(exc)}
        step.outcome = {"error": str(exc)}

    if deps.ai_observer is not None:
        await deps.ai_observer.capture_tool_call(
            tool_id=tool_id, params=params, result=result,
            metadata={
                "correlation_id": session.correlation_id,
                "session_id": str(session.id),
                "workspace_id": str(session.workspace_id),
                "openspec_id": session.openspec_id,
                "actor": session.originator_principal,
                "agent_mode": True,
            },
            latency_ms=(time.perf_counter() - started) * 1000,
            error=error,
        )

    decision = await _record_decision(
        deps, session, step, tool_id, params,
        outcome=outcome,
        policy_evaluated=policy,
        outcome_detail=outcome_detail,
    )
    step.decision_id = decision.id

    if outcome == "failed":
        raise RuntimeError(str(outcome_detail))


def _kind_action_class(kind: StepKind, tool_id: str) -> str:
    if kind == "workflow":
        return "workflow:dispatch"
    if kind == "agent":
        return "agent:delegate"
    if kind == "approval":
        return "approval:request"
    return _action_class_for(tool_id)


async def _record_decision(
    deps: ExecutorDeps,
    session: AgentModeSession,
    step: AgentModeStep,
    tool_id: str,
    params: dict[str, Any],
    *,
    outcome: str,
    policy_evaluated: dict[str, Any] | None,
    outcome_detail: dict[str, Any] | None,
) -> DecisionRecord:
    # Agent-mode reuses the alfred_decision table via a shadow alfred_session
    # row keyed off the agent-mode session id. Decisions store the same
    # correlation_id so audit joins are trivial.
    rec = DecisionRecord(
        session_id=session.id,  # shared id space — agent-mode session id
        workspace_id=session.workspace_id,
        actor=session.originator_principal,
        correlation_id=session.correlation_id,
        intent=step.summary or "agent-mode step",
        tool_kind="mcp" if tool_id.startswith("mcp:") else "skill",
        tool_id=tool_id,
        params_redacted=redact(params),
        policy_evaluated=policy_evaluated,
        outcome=outcome,  # type: ignore[arg-type]
        outcome_detail=outcome_detail,
    )
    try:
        await deps.store.append_decision(rec)
    except Exception as exc:
        log.warning("decision_persist_failed", error=str(exc))
    return rec


async def _emit(
    deps: ExecutorDeps,
    session: AgentModeSession,
    event_type: str,
    payload: dict[str, Any],
) -> None:
    if deps.emit_event is None:
        return
    enriched = {
        "session_id": str(session.id),
        "workspace_id": str(session.workspace_id),
        "correlation_id": session.correlation_id,
        "openspec_id": session.openspec_id,
        "model_id": session.model_id,
        **payload,
    }
    try:
        await deps.emit_event(event_type, enriched)
    except Exception as exc:
        log.warning("event_emit_failed", event=event_type, error=str(exc))
