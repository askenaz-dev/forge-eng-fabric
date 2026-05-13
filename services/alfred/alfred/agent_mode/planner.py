"""Build an initial plan from a committed OpenSpec.

Uses RAG to retrieve relevant docs and the existing LiteLLM client to ask the
model for an ordered list of steps. The plan schema matches design.md D2.

If the LLM is unreachable or returns malformed JSON the planner falls back to
the canonical intent-to-deploy plan so we never block a session on planner
failure — the session still goes through the same per-step policy/permission
gates as a wizard call.
"""

from __future__ import annotations

import json
import re
import uuid
from collections.abc import Sequence
from typing import Any

from alfred.gateways import OpenSpecClient, RAGClient
from alfred.llm import LiteLLMClient
from alfred.logging import get_logger

log = get_logger(__name__)

PLANNER_SYSTEM_PROMPT = (
    "You are Alfred's agent-mode planner. Given a committed OpenSpec, return an "
    "ordered JSON plan to take the spec from scaffold to deployed application. "
    "Respond ONLY in JSON of the form: "
    '{"thought": str, "steps": ['
    '{"idx": int, "kind": "tool"|"workflow"|"agent"|"final", '
    '"tool_id": str|null, "workflow_id": str|null, "agent_id": str|null, '
    '"criticality": "low"|"medium"|"high"|"critical", '
    '"summary": str, "params": object}'
    "]}. "
    "Prefer dispatching the `forge.reference.intent-to-deploy@1` workflow "
    "rather than re-implementing scaffold/CI/deploy step-by-step. "
    "Never include code fences."
)

# Canonical fallback plan — drives the reference intent-to-deploy workflow.
CANONICAL_PLAN: dict[str, Any] = {
    "thought": "Fallback to the canonical intent-to-deploy reference workflow.",
    "steps": [
        {
            "idx": 0,
            "kind": "workflow",
            "tool_id": None,
            "workflow_id": "forge.reference.intent-to-deploy@1",
            "agent_id": None,
            "criticality": "high",
            "summary": "Scaffold, open PR, run CI, request pre-prod approval and deploy.",
            "params": {},
        },
        {
            "idx": 1,
            "kind": "final",
            "tool_id": None,
            "workflow_id": None,
            "agent_id": None,
            "criticality": "low",
            "summary": "Report PR URL, CI status and deploy URL back to the originator.",
            "params": {},
        },
    ],
}


def _parse_plan_json(content: str) -> dict[str, Any]:
    stripped = content.strip()
    fence = re.match(r"^```(?:json)?\s*([\s\S]*?)\s*```$", stripped, re.IGNORECASE)
    if fence:
        stripped = fence.group(1)
    try:
        plan = json.loads(stripped)
    except json.JSONDecodeError:
        return dict(CANONICAL_PLAN)
    if not isinstance(plan, dict) or not isinstance(plan.get("steps"), list):
        return dict(CANONICAL_PLAN)
    return plan


def _normalize_steps(plan: dict[str, Any]) -> dict[str, Any]:
    """Ensure each step has an idx and recognized kind/criticality."""
    valid_kinds = {"tool", "workflow", "agent", "approval", "final"}
    valid_crit = {"low", "medium", "high", "critical"}
    steps: list[dict[str, Any]] = []
    for i, raw in enumerate(plan.get("steps") or []):
        if not isinstance(raw, dict):
            continue
        kind = raw.get("kind") if raw.get("kind") in valid_kinds else "tool"
        criticality = raw.get("criticality") if raw.get("criticality") in valid_crit else "low"
        steps.append(
            {
                "idx": int(raw.get("idx", i)) if isinstance(raw.get("idx", i), (int, float)) else i,
                "kind": kind,
                "tool_id": raw.get("tool_id") or None,
                "workflow_id": raw.get("workflow_id") or None,
                "agent_id": raw.get("agent_id") or None,
                "criticality": criticality,
                "summary": str(raw.get("summary") or ""),
                "params": raw.get("params") or {},
            }
        )
    plan["steps"] = steps
    return plan


async def build_initial_plan(
    *,
    workspace_id: uuid.UUID,
    openspec_id: str,
    intent: str,
    correlation_id: str,
    llm: LiteLLMClient,
    rag: RAGClient,
    openspec: OpenSpecClient,
    model: str,
    rag_top_k: int = 8,
) -> dict[str, Any]:
    """Return a plan dict matching D2. Falls back to CANONICAL_PLAN on error."""

    spec = await openspec.get(openspec_id)
    rag_chunks: Sequence[dict[str, Any]] = await rag.query(
        workspace_id=workspace_id, text=intent, top_k=rag_top_k, principal="alfred"
    )
    user_prompt_parts = [f"intent: {intent}", f"openspec_id: {openspec_id}"]
    if spec:
        user_prompt_parts.append("openspec: " + json.dumps(spec)[:4000])
    if rag_chunks:
        snippets = "\n\n".join(
            f"[{c.get('source_ref', '?')}] {str(c.get('text', ''))[:600]}"
            for c in rag_chunks[:rag_top_k]
        )
        user_prompt_parts.append("references:\n" + snippets)
    messages = [
        {"role": "system", "content": PLANNER_SYSTEM_PROMPT},
        {"role": "user", "content": "\n\n".join(user_prompt_parts)},
    ]

    try:
        completion = await llm.chat(
            model=model,
            messages=messages,
            metadata={
                "correlation_id": correlation_id,
                "actor": "alfred",
                "phase": "agent_mode.plan",
                "openspec_id": openspec_id,
            },
        )
        content = (
            completion.get("choices", [{}])[0].get("message", {}).get("content") or ""
        )
        plan = _parse_plan_json(content)
    except Exception as exc:
        log.warning("planner_llm_failed", error=str(exc))
        plan = dict(CANONICAL_PLAN)

    if not plan.get("steps"):
        plan = dict(CANONICAL_PLAN)
    return _normalize_steps(plan)


def replan(
    current_plan: dict[str, Any], failed_step_idx: int, reason: str
) -> dict[str, Any]:
    """Insert a fix step after the failed step and renumber subsequent steps.

    Returns a new plan dict; does not mutate the caller's plan. The fix step is
    a `tool` kind placeholder — the executor decides how to dispatch it.
    """
    new_steps: list[dict[str, Any]] = []
    inserted = False
    for raw in current_plan.get("steps") or []:
        step = dict(raw)
        new_steps.append(step)
        if step.get("idx") == failed_step_idx and not inserted:
            new_steps.append(
                {
                    "idx": failed_step_idx + 1,
                    "kind": "tool",
                    "tool_id": "skill:diagnose-and-retry",
                    "workflow_id": None,
                    "agent_id": None,
                    "criticality": "medium",
                    "summary": f"Diagnose and retry failed step {failed_step_idx}: {reason}",
                    "params": {"failed_idx": failed_step_idx, "reason": reason},
                }
            )
            inserted = True
    # Renumber idx values to stay monotone after insertion.
    for i, step in enumerate(new_steps):
        step["idx"] = i
    return {
        "thought": current_plan.get("thought", "") + f"\nReplan after step {failed_step_idx}: {reason}",
        "steps": new_steps,
    }
