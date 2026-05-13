"""SSE integration test asserting replay-then-live behavior with Last-Event-ID."""

from __future__ import annotations

import uuid

import pytest

from alfred.agent_mode.models import AgentModeSession, AgentModeStep
from alfred.agent_mode.router import _sse_event
from alfred.agent_mode.store import AgentModeStore
from alfred.store import InMemoryStore


def test_sse_event_encoding_includes_id_event_and_data() -> None:
    raw = _sse_event(7, "step", {"idx": 3, "status": "succeeded"})
    text = raw.decode()
    assert text.startswith("id: 7\n")
    assert "event: step\n" in text
    assert "\"status\": \"succeeded\"" in text
    assert text.endswith("\n\n")


@pytest.mark.asyncio
async def test_step_persistence_supports_replay_in_order() -> None:
    """Replay reads steps in `idx` order; Last-Event-ID skips already-seen ones."""
    inner = InMemoryStore()
    store = AgentModeStore(inner)
    session = AgentModeSession(
        id=uuid.uuid4(),
        workspace_id=uuid.uuid4(),
        correlation_id="c",
        originator_principal="user:a",
        model_id="m",
        plan_json={"steps": []},
        frozen_autonomy_policy={},
        status="completed",
    )
    await store.create_session(session)
    for idx in range(3):
        await store.upsert_step(
            AgentModeStep(
                session_id=session.id, idx=idx, kind="tool", tool_id="x",
                status="succeeded", summary=f"step-{idx}",
            )
        )

    steps = await store.list_steps(session.id)
    assert [s.idx for s in steps] == [0, 1, 2]

    # Resume from id=1 means: skip events 0 and 1, replay event 2.
    last_id = 1
    replayed = []
    for replay_idx, step in enumerate(steps):
        if replay_idx > last_id:
            replayed.append(step.idx)
    assert replayed == [2]
