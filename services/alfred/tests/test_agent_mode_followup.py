"""Follow-up + frozen-preset tests for agent-mode."""

from __future__ import annotations

import uuid

import pytest

from alfred.agent_mode.models import AgentModeSession
from alfred.agent_mode.store import AgentModeStore
from alfred.autonomy_presets import validate_override
from alfred.store import InMemoryStore


@pytest.mark.asyncio
async def test_in_ceiling_followup_is_appended_to_plan() -> None:
    """Appending a routine follow-up to a session under autonomous ceiling
    extends the plan with a new tool step."""
    inner = InMemoryStore()
    store = AgentModeStore(inner)
    session = AgentModeSession(
        workspace_id=uuid.uuid4(),
        correlation_id="c",
        originator_principal="user:a",
        model_id="m",
        plan_json={"steps": [{"idx": 0, "kind": "final", "summary": "done", "params": {}}]},
        frozen_autonomy_policy={"ceilings": {"alfred:agent-mode.run": "autonomous"}},
        status="completed",
    )
    await store.create_session(session)

    plan = dict(session.plan_json)
    steps = list(plan["steps"])
    steps.append(
        {
            "idx": len(steps),
            "kind": "tool",
            "tool_id": "skill:follow-up",
            "criticality": "low",
            "summary": "do another thing",
            "params": {"intent": "do another thing"},
        }
    )
    plan["steps"] = steps
    updated = await store.update_session(session.id, plan_json=plan)
    assert updated.plan_json["steps"][-1]["tool_id"] == "skill:follow-up"


def test_ceiling_breaching_followup_is_rejected_by_validator() -> None:
    """A `restricted` ceiling rejects any override on that action class."""
    preset = {
        "ceilings": {
            "alfred:agent-mode.run": "restricted",
        }
    }
    ok, reason = validate_override(preset, "alfred:agent-mode.run", "autonomous")
    assert not ok
    assert reason is not None and "ceiling" in reason


@pytest.mark.asyncio
async def test_frozen_preset_in_session_is_not_overwritten_by_admin_edits() -> None:
    """Per D4 — session.frozen_autonomy_policy is the source of truth at start."""
    inner = InMemoryStore()
    store = AgentModeStore(inner)
    frozen = {"name": "full-autonomy", "ceilings": {"alfred:agent-mode.run": "autonomous"}}
    session = AgentModeSession(
        workspace_id=uuid.uuid4(),
        correlation_id="c",
        originator_principal="user:a",
        model_id="m",
        plan_json={"steps": []},
        frozen_autonomy_policy=frozen,
        status="running",
    )
    await store.create_session(session)
    # Admin edits the workspace preset to be tighter. The session row stays.
    refreshed = await store.get_session(session.id)
    assert refreshed.frozen_autonomy_policy["ceilings"]["alfred:agent-mode.run"] == "autonomous"
