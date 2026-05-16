"""Alfred reasoning/action loop.

The full ADK-style loop is intentionally compact in Phase 1: per iteration we
1. retrieve relevant RAG chunks,
2. structure the prompt with guardrails,
3. ask the LLM (via LiteLLM) for the next step,
4. evaluate policy and required permissions,
5. either execute a tool, request approval, or finalize.

Each step writes one or more decision records.
"""

from __future__ import annotations

import json
import re
import time
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import datetime
from typing import Any

from alfred.gateways import (
    ApprovalsClient,
    OpenSpecClient,
    PermissionsClient,
    PolicyClient,
    RAGClient,
)
from alfred.guardrails import Guardrails
from alfred.llm import LiteLLMClient, RequestContext
from alfred.logging import get_logger
from alfred.models import DecisionRecord, IntentResponse, Session
from alfred.observability import AIObserver
from alfred.redaction import redact
from alfred.store import Store

log = get_logger(__name__)

ToolHandler = Callable[[str, dict[str, Any]], Awaitable[dict[str, Any]]]


@dataclass
class LoopDeps:
    store: Store
    llm: LiteLLMClient
    rag: RAGClient
    policy: PolicyClient
    approvals: ApprovalsClient
    permissions: PermissionsClient
    openspec: OpenSpecClient
    guardrails: Guardrails
    tool_handler: ToolHandler
    default_model: str
    rag_top_k: int = 8
    max_iterations: int = 8
    ai_observer: AIObserver | None = None


SYSTEM_PROMPT = (
    "You are Alfred, the Forge Engineering Fabric Control Plane Agent. "
    "Operate autonomously by default within delegated permissions. "
    "Always cite OpenSpecs, runbooks and policies when reasoning. "
    "Respond ONLY in JSON of the form: "
    '{"thought": str, "next_action": '
    '{"kind": "tool"|"final"|"approval", "tool_id": str|null, "params": object|null, '
    '"summary": str, "criticality": "low"|"medium"|"high"|"critical"} } '
    "Never include code fences in your reply."
)


def _parse_llm_json(content: str) -> dict[str, Any]:
    # Strip code fences if a model insists on them.
    stripped = content.strip()
    fence = re.match(r"^```(?:json)?\s*([\s\S]*?)\s*```$", stripped, re.IGNORECASE)
    if fence:
        stripped = fence.group(1)
    try:
        return json.loads(stripped)
    except json.JSONDecodeError:
        return {
            "thought": "(non-JSON model output, treated as final)",
            "next_action": {
                "kind": "final",
                "tool_id": None,
                "params": None,
                "summary": stripped[:500],
                "criticality": "low",
            },
        }


async def run_intent(
    deps: LoopDeps,
    *,
    actor: str,
    workspace_id: uuid.UUID,
    intent: str,
    correlation_id: str,
    openspec_id: str | None = None,
    metadata: dict[str, Any] | None = None,
    tenant_id: str = "",
    data_classification: str = "internal",
) -> IntentResponse:
    """Run a single intent through the loop. Persists session + decisions.

    `tenant_id` and `data_classification` flow into every LiteLLM call
    via `RequestContext` (alfred-litellm-header-injection G1). The HTTP
    layer derives `tenant_id` from the JWT/principal; tests pass a fixed
    value. Missing tenant_id causes LiteLLMClient to fail closed.
    """

    session = Session(
        id=uuid.uuid4(),
        workspace_id=workspace_id,
        actor=actor,
        started_at=datetime.utcnow(),
        last_activity_at=datetime.utcnow(),
        status="open",
        correlation_id=correlation_id,
        metadata=metadata or {},
        tenant_id=tenant_id,
    )
    llm_context = RequestContext(
        tenant_id=tenant_id,
        workspace_id=str(workspace_id),
        correlation_id=correlation_id,
        data_classification=data_classification,
    )
    await deps.store.create_session(session)
    await deps.store.append_message(
        session_id=session.id, role="user", content=intent, tool_call_id=None
    )

    decisions: list[DecisionRecord] = []
    final_message = ""
    requires_approval = False
    approval_request_id: str | None = None

    rag_results = await deps.rag.query(
        workspace_id=workspace_id, text=intent, top_k=deps.rag_top_k, principal=actor
    )
    rec_retrieve = DecisionRecord(
        session_id=session.id,
        workspace_id=workspace_id,
        actor=actor,
        correlation_id=correlation_id,
        intent=intent,
        retrieved_refs=[
            {"source_ref": r.get("source_ref"), "score": r.get("score"), "chunk_id": r.get("chunk_id")}
            for r in rag_results
        ],
        tool_kind=None,
        tool_id=None,
        outcome="succeeded",
        outcome_detail={"chunks": len(rag_results)},
    )
    await deps.store.append_decision(rec_retrieve)
    decisions.append(rec_retrieve)

    messages = deps.guardrails.build_messages(
        system_prompt=SYSTEM_PROMPT, user_intent=intent, rag_chunks=rag_results
    )
    history = list(messages)

    for i in range(deps.max_iterations):
        completion = await deps.llm.chat(
            model=deps.default_model,
            messages=history,
            context=llm_context,
            metadata={
                "correlation_id": correlation_id,
                "session_id": str(session.id),
                "actor": actor,
                "iteration": i,
            },
        )
        choice = completion.get("choices", [{}])[0].get("message", {})
        content = choice.get("content") or ""
        await deps.store.append_message(
            session_id=session.id, role="assistant", content=content, tool_call_id=None
        )
        history.append({"role": "assistant", "content": content})
        plan = _parse_llm_json(content)
        action = plan.get("next_action") or {}
        kind = action.get("kind", "final")

        if kind == "final":
            final_message = action.get("summary") or content
            rec = DecisionRecord(
                session_id=session.id,
                workspace_id=workspace_id,
                actor=actor,
                correlation_id=correlation_id,
                intent=intent,
                tool_kind="llm",
                tool_id=deps.default_model,
                params_redacted=redact({"thought": plan.get("thought")}),
                outcome="succeeded",
                outcome_detail={"final": True, "iteration": i},
            )
            await deps.store.append_decision(rec)
            decisions.append(rec)
            break

        tool_id = str(action.get("tool_id") or "")
        params = action.get("params") or {}
        criticality = str(action.get("criticality") or "low")

        # Permission gate
        perm = await deps.permissions.can(
            subject="alfred",
            action_class=_action_class_for(tool_id),
            scope_kind="workspace",
            scope_id=str(workspace_id),
            criticality=criticality,
        )
        if not perm.get("allowed"):
            rec = DecisionRecord(
                session_id=session.id,
                workspace_id=workspace_id,
                actor=actor,
                correlation_id=correlation_id,
                intent=intent,
                tool_kind="mcp" if tool_id.startswith("mcp:") else "skill",
                tool_id=tool_id,
                params_redacted=redact(params),
                policy_evaluated={"permissions": perm},
                outcome="failed",
                outcome_detail={"reason": "no delegated permission", "perm": perm},
            )
            await deps.store.append_decision(rec)
            decisions.append(rec)
            final_message = (
                f"Cannot perform `{tool_id}` — Alfred lacks a delegated permission "
                f"with action_class `{_action_class_for(tool_id)}` on this workspace. "
                f"Reason: {perm.get('reason')}."
            )
            break

        # Policy gate
        policy = await deps.policy.evaluate(
            principal="alfred",
            action=tool_id,
            workspace_id=workspace_id,
            openspec_id=openspec_id,
            target=params,
            env=str(metadata.get("env", "dev") if metadata else "dev"),
            criticality=criticality,
        )
        decision = policy.get("decision", "allow")
        if decision == "deny":
            rec = DecisionRecord(
                session_id=session.id,
                workspace_id=workspace_id,
                actor=actor,
                correlation_id=correlation_id,
                intent=intent,
                tool_kind="mcp" if tool_id.startswith("mcp:") else "skill",
                tool_id=tool_id,
                params_redacted=redact(params),
                policy_evaluated=policy,
                outcome="failed",
                outcome_detail={"reason": "policy:deny"},
            )
            await deps.store.append_decision(rec)
            decisions.append(rec)
            final_message = f"Policy denies `{tool_id}`: {policy.get('rationale')}."
            break
        if decision == "requires_approval":
            req = await deps.approvals.request(
                principal="alfred",
                action=tool_id,
                workspace_id=workspace_id,
                openspec_id=openspec_id,
                target=params,
                rationale=plan.get("thought") or "Alfred-initiated action",
                required_approvers=policy.get("required_approvers") or ["workspace:owner"],
                criticality=criticality,
                correlation_id=correlation_id,
            )
            requires_approval = True
            approval_request_id = req.get("id")
            rec = DecisionRecord(
                session_id=session.id,
                workspace_id=workspace_id,
                actor=actor,
                correlation_id=correlation_id,
                intent=intent,
                tool_kind="mcp" if tool_id.startswith("mcp:") else "skill",
                tool_id=tool_id,
                params_redacted=redact(params),
                policy_evaluated=policy,
                outcome="approved" if req.get("status") == "approved" else "pending",
                outcome_detail={"approval": req},
            )
            await deps.store.append_decision(rec)
            decisions.append(rec)
            final_message = (
                f"Action `{tool_id}` requires approval ({approval_request_id}). "
                f"Halted — will resume once an approver decides."
            )
            break

        # Allow → execute the tool
        started = time.perf_counter()
        result: dict[str, Any]
        error: str | None = None
        try:
            result = await deps.tool_handler(tool_id, params)
            outcome = "succeeded"
            outcome_detail: dict[str, Any] = {"result_preview": str(result)[:500]}
        except Exception as exc:
            outcome = "failed"
            error = str(exc)
            outcome_detail = {"error": str(exc)}
            result = {"error": str(exc)}
        if deps.ai_observer:
            await deps.ai_observer.capture_tool_call(
                tool_id=tool_id,
                params=params,
                result=result,
                metadata={
                    "correlation_id": correlation_id,
                    "session_id": str(session.id),
                    "workspace_id": str(workspace_id),
                    "openspec_id": openspec_id,
                    "actor": actor,
                },
                latency_ms=(time.perf_counter() - started) * 1000,
                error=error,
            )
        rec = DecisionRecord(
            session_id=session.id,
            workspace_id=workspace_id,
            actor=actor,
            correlation_id=correlation_id,
            intent=intent,
            tool_kind="mcp" if tool_id.startswith("mcp:") else "skill",
            tool_id=tool_id,
            params_redacted=redact(params),
            policy_evaluated=policy,
            outcome=outcome,
            outcome_detail=outcome_detail,
        )
        await deps.store.append_decision(rec)
        decisions.append(rec)
        await deps.store.append_message(
            session_id=session.id,
            role="tool",
            content=json.dumps(redact(result), default=str),
            tool_call_id=tool_id,
        )
        history.append({"role": "tool", "content": json.dumps(redact(result), default=str)})

        if openspec_id:
            await deps.openspec.append_decision(
                openspec_id,
                {
                    "actor": "alfred",
                    "decision": f"executed {tool_id}",
                    "rationale": plan.get("thought") or "",
                    "policy": policy,
                    "correlation_id": correlation_id,
                    "occurred_at": datetime.utcnow().isoformat() + "Z",
                },
            )

    if not final_message:
        final_message = "(loop reached max iterations without final answer)"
    return IntentResponse(
        session_id=session.id,
        correlation_id=correlation_id,
        decisions=decisions,
        final_message=final_message,
        requires_approval=requires_approval,
        approval_request_id=approval_request_id,
    )


def _action_class_for(tool_id: str) -> str:
    """Map a tool_id to a coarse action_class used for permission checks."""

    if tool_id.startswith("mcp:openspec"):
        return "openspec:write"
    if tool_id.startswith("mcp:github"):
        return "github:read"
    if tool_id.startswith("mcp:jira"):
        return "jira:write"
    if tool_id.startswith("mcp:confluence"):
        return "confluence:write"
    if tool_id.startswith("skill:"):
        return "skill:invoke"
    return "tool:invoke"
