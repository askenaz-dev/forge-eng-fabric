"""End-to-end test driving a canned OpenSpec through scaffold → PR → CI →
HITL → deploy with a mocked workflow-runtime.

Asserts the milestone event ordering from the spec scenario:
  session_started → step_started (workflow) → paused_for_approval → resumed
  → step_completed → completed.
"""

from __future__ import annotations

import uuid
from typing import Any

import pytest

from alfred.agent_mode.executor import ExecutorDeps, execute_session, resume
from alfred.agent_mode.models import AgentModeSession
from alfred.agent_mode.store import AgentModeStore
from alfred.store import InMemoryStore


class FakeLLM:
    async def chat(self, **_kwargs):
        return {"choices": [{"message": {"content": "{}"}}]}


class FakeRAG:
    async def query(self, **_kwargs):
        return []


class FakeOpenSpec:
    async def get(self, _id):
        return None

    async def append_decision(self, *_a, **_k):
        return True


class FakeAllowingPolicy:
    async def evaluate(self, **kwargs):
        action = kwargs.get("action") or ""
        criticality = kwargs.get("criticality") or "low"
        if "deploy" in action or criticality == "high":
            return {"decision": "requires_approval", "rationale": "deploy gate"}
        return {"decision": "allow", "rationale": "ok"}


class FakePermissions:
    async def can(self, **_kwargs):
        return {"allowed": True, "reason": "test"}


class FakeApprovals:
    def __init__(self) -> None:
        self.status = "pending"
        self.requested = 0

    async def request(self, **_kwargs):
        self.requested += 1
        return {"id": "approval-1", "status": self.status}


class FakeWorkflowDispatcher:
    def __init__(self) -> None:
        self.calls: list[tuple[str, dict[str, Any], str]] = []

    async def dispatch(self, workflow_id: str, params: dict[str, Any], correlation_id: str):
        self.calls.append((workflow_id, params, correlation_id))
        return {"run_id": "run-1", "events": ["scaffold", "pr", "ci-green"]}


class EventCapture:
    def __init__(self) -> None:
        self.events: list[tuple[str, dict[str, Any]]] = []

    async def emit(self, event_type: str, payload: dict[str, Any]) -> None:
        self.events.append((event_type, payload))


async def _tool(_tool_id: str, _params: dict[str, Any]) -> dict[str, Any]:
    return {"ok": True}


@pytest.mark.asyncio
async def test_intent_to_deploy_milestones_in_order() -> None:
    inner = InMemoryStore()
    agent_store = AgentModeStore(inner)
    events = EventCapture()
    approvals = FakeApprovals()
    dispatcher = FakeWorkflowDispatcher()

    deps = ExecutorDeps(
        store=inner,
        agent_store=agent_store,
        llm=FakeLLM(),
        rag=FakeRAG(),
        policy=FakeAllowingPolicy(),
        approvals=approvals,
        permissions=FakePermissions(),
        openspec=FakeOpenSpec(),
        tool_handler=_tool,
        workflow_dispatcher=dispatcher.dispatch,
        budget_probe=None,
        emit_event=events.emit,
    )

    session = AgentModeSession(
        workspace_id=uuid.uuid4(),
        openspec_id="spec-demo",
        correlation_id="corr-demo",
        originator_principal="user:alice",
        model_id="test-model",
        plan_json={
            "thought": "canonical intent-to-deploy",
            "steps": [
                {
                    "idx": 0,
                    "kind": "workflow",
                    "workflow_id": "forge.reference.intent-to-deploy@1",
                    "criticality": "high",
                    "summary": "Run intent-to-deploy",
                    "params": {},
                },
                {
                    "idx": 1,
                    "kind": "final",
                    "summary": "Done — PR merged, deploy green.",
                    "params": {},
                },
            ],
        },
        frozen_autonomy_policy={"ceilings": {}},
        status="planning",
    )
    await agent_store.create_session(session)

    # First pass — high criticality → paused at workflow step.
    first = await execute_session(deps, session)
    assert first.status == "paused_for_approval"

    # Approve and resume.
    approvals.status = "approved"
    final = await resume(deps, session.id)
    assert final.status == "completed"
    assert dispatcher.calls and dispatcher.calls[0][0] == "forge.reference.intent-to-deploy@1"

    types = [e[0] for e in events.events]
    # Required milestones in correct order:
    expected_order = [
        "alfred.agent_mode.session_started.v1",
        "alfred.agent_mode.step_started.v1",
        "alfred.agent_mode.paused_for_approval.v1",
        "alfred.agent_mode.resumed.v1",
        "alfred.agent_mode.step_started.v1",
        "alfred.agent_mode.step_completed.v1",
        "alfred.agent_mode.completed.v1",
    ]
    # Each expected event appears in `types` in order (we permit extras).
    pos = 0
    for expected in expected_order:
        try:
            pos = types.index(expected, pos) + 1
        except ValueError:
            pytest.fail(f"missing or out-of-order event {expected}; got {types}")
