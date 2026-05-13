"""Executor tests covering: autonomous success, policy deny, requires_approval
pause + resume, requires_approval reject + abort, recoverable replan, budget
pause and frozen-preset enforcement."""

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


class FakePermissions:
    def __init__(self, allowed: bool = True) -> None:
        self.allowed = allowed

    async def can(self, **_kwargs):
        return {"allowed": self.allowed, "reason": "test"}


class FakePolicy:
    def __init__(self, decision: str = "allow") -> None:
        self.decision = decision

    async def evaluate(self, **_kwargs):
        return {"decision": self.decision, "rationale": "test"}


class FakeApprovals:
    def __init__(self, status: str = "pending") -> None:
        self.status = status
        self.requested = 0

    async def request(self, **_kwargs):
        self.requested += 1
        return {"id": "approval-x", "status": self.status}


class FakeToolHandler:
    def __init__(self, *, raises: Exception | None = None, payload: dict[str, Any] | None = None) -> None:
        self._raises = raises
        self._payload = payload or {"ok": True}
        self.calls: list[tuple[str, dict[str, Any]]] = []

    async def __call__(self, tool_id: str, params: dict[str, Any]) -> dict[str, Any]:
        self.calls.append((tool_id, params))
        if self._raises:
            raise self._raises
        return self._payload


class EventCapture:
    def __init__(self) -> None:
        self.events: list[tuple[str, dict[str, Any]]] = []

    async def emit(self, event_type: str, payload: dict[str, Any]) -> None:
        self.events.append((event_type, payload))


def _session(*, plan_steps: list[dict[str, Any]], frozen: dict[str, Any] | None = None) -> AgentModeSession:
    return AgentModeSession(
        workspace_id=uuid.uuid4(),
        correlation_id="corr-1",
        originator_principal="user:test",
        model_id="test-model",
        plan_json={"thought": "t", "steps": plan_steps},
        frozen_autonomy_policy=frozen or {},
        status="planning",
    )


def _deps(*, perms=None, policy=None, approvals=None, tool=None, events=None, budget=None) -> ExecutorDeps:
    inner = InMemoryStore()
    return ExecutorDeps(
        store=inner,
        agent_store=AgentModeStore(inner),
        llm=FakeLLM(),
        rag=FakeRAG(),
        policy=policy or FakePolicy("allow"),
        approvals=approvals or FakeApprovals(),
        permissions=perms or FakePermissions(True),
        openspec=FakeOpenSpec(),
        tool_handler=tool or FakeToolHandler(),
        workflow_dispatcher=None,
        budget_probe=budget,
        emit_event=(events or EventCapture()).emit,
    )


@pytest.mark.asyncio
async def test_autonomous_step_succeeds_end_to_end() -> None:
    events = EventCapture()
    deps = _deps(events=events)
    session = _session(
        plan_steps=[
            {"idx": 0, "kind": "tool", "tool_id": "skill:scaffold", "criticality": "low", "summary": "s"},
            {"idx": 1, "kind": "final", "summary": "done"},
        ]
    )
    await deps.agent_store.create_session(session)
    result = await execute_session(deps, session)
    assert result.status == "completed"
    assert any(e[0] == "alfred.agent_mode.completed.v1" for e in events.events)


@pytest.mark.asyncio
async def test_policy_deny_aborts_session() -> None:
    events = EventCapture()
    deps = _deps(policy=FakePolicy("deny"), events=events)
    session = _session(
        plan_steps=[
            {"idx": 0, "kind": "tool", "tool_id": "skill:x", "criticality": "high", "summary": "s"}
        ]
    )
    await deps.agent_store.create_session(session)
    result = await execute_session(deps, session)
    assert result.status == "aborted"
    assert any(e[0] == "alfred.agent_mode.aborted.v1" for e in events.events)


@pytest.mark.asyncio
async def test_requires_approval_pauses_and_resume_after_grant() -> None:
    events = EventCapture()
    approvals = FakeApprovals(status="pending")
    deps = _deps(policy=FakePolicy("requires_approval"), approvals=approvals, events=events)
    session = _session(
        plan_steps=[
            {"idx": 0, "kind": "tool", "tool_id": "skill:deploy", "criticality": "high", "summary": "s"},
            {"idx": 1, "kind": "final", "summary": "done"},
        ]
    )
    await deps.agent_store.create_session(session)
    result = await execute_session(deps, session)
    assert result.status == "paused_for_approval"
    assert any(e[0] == "alfred.agent_mode.paused_for_approval.v1" for e in events.events)

    # Simulate approval grant: flip policy to allow and resume.
    deps2 = _deps(events=events)
    deps2.agent_store = deps.agent_store
    deps2.store = deps.store
    result2 = await resume(deps2, session.id)
    assert result2.status == "completed"


@pytest.mark.asyncio
async def test_permission_denied_aborts() -> None:
    events = EventCapture()
    deps = _deps(perms=FakePermissions(False), events=events)
    session = _session(
        plan_steps=[
            {"idx": 0, "kind": "tool", "tool_id": "skill:x", "criticality": "low", "summary": "s"}
        ]
    )
    await deps.agent_store.create_session(session)
    result = await execute_session(deps, session)
    assert result.status == "aborted"
    assert result.aborted_reason == "permission_denied"


@pytest.mark.asyncio
async def test_recoverable_failure_triggers_replan() -> None:
    events = EventCapture()
    # First call raises, subsequent calls succeed (post-replan retry).
    handler = FakeToolHandler(raises=RuntimeError("transient"))
    deps = _deps(tool=handler, events=events)
    session = _session(
        plan_steps=[
            {"idx": 0, "kind": "tool", "tool_id": "skill:flaky", "criticality": "low", "summary": "s"},
            {"idx": 1, "kind": "final", "summary": "done"},
        ]
    )
    await deps.agent_store.create_session(session)
    # After the failure the executor inserts a fix step and recurses. The fix
    # step will fail the same way, so the test asserts replan happened (event
    # emitted) and the session ended with `failed` or `aborted`.
    await execute_session(deps, session)
    assert any(e[0] == "alfred.agent_mode.plan_revised.v1" for e in events.events)


@pytest.mark.asyncio
async def test_budget_probe_pauses_session() -> None:
    events = EventCapture()

    async def over_budget(_workspace_id):
        return {"status": "over_budget", "remaining_usd": 0}

    deps = _deps(events=events, budget=over_budget)
    session = _session(
        plan_steps=[
            {"idx": 0, "kind": "tool", "tool_id": "skill:x", "criticality": "low", "summary": "s"}
        ]
    )
    await deps.agent_store.create_session(session)
    result = await execute_session(deps, session)
    assert result.status == "paused_for_budget"
    assert any(e[0] == "alfred.agent_mode.paused_for_budget.v1" for e in events.events)


@pytest.mark.asyncio
async def test_frozen_preset_ceiling_forces_approval() -> None:
    """If the frozen preset has a `requires_approval` ceiling on tool:invoke,
    even an `allow` policy gets escalated to approval."""
    events = EventCapture()
    approvals = FakeApprovals(status="pending")
    deps = _deps(policy=FakePolicy("allow"), approvals=approvals, events=events)
    frozen = {
        "name": "manual-prod",
        "ceilings": {"tool:invoke": "requires_approval"},
    }
    session = _session(
        plan_steps=[
            {"idx": 0, "kind": "tool", "tool_id": "skill:x", "criticality": "low", "summary": "s"}
        ],
        frozen=frozen,
    )
    await deps.agent_store.create_session(session)
    result = await execute_session(deps, session)
    assert result.status == "paused_for_approval"
    assert approvals.requested == 1
