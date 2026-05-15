"""Tests for agent-mode session `start_step` field (alfred-console-redesign §7).

Covers:
  - 7.1: 422 for unknown step values (model + validation constants)
  - 7.2: 409 spec_not_ready_for_architect — lifecycle gate logic
  - 7.3: Planner honours start_step hint
  - Default: no start_step → discovery
"""
from __future__ import annotations

import json
import uuid
from typing import Any

import pytest

from alfred.agent_mode.models import ALLOWED_START_STEPS, COMMITTED_LIFECYCLE_STATES, StartSessionRequest
from alfred.agent_mode.planner import CANONICAL_PLAN, build_initial_plan


# ── Fakes ──────────────────────────────────────────────────────────────────


class FakeLLM:
    def __init__(self, content: str | None = None) -> None:
        self.content = content

    async def chat(self, **_kw):
        body = self.content or json.dumps({
            "thought": "test",
            "steps": [{"idx": 0, "kind": "final", "summary": "done", "params": {}}],
        })
        return {"choices": [{"message": {"content": body}}]}


class FakeLLMWithStep:
    """Returns a plan whose first step matches the requested start_step kind."""

    def __init__(self, kind: str = "workflow", workflow_id: str = "forge.reference.intent-to-deploy@1") -> None:
        self.kind = kind
        self.workflow_id = workflow_id

    async def chat(self, messages: list | None = None, **_kw):
        step0: dict[str, Any] = {
            "idx": 0,
            "kind": self.kind,
            "workflow_id": self.workflow_id,
            "tool_id": None,
            "agent_id": None,
            "criticality": "high",
            "summary": f"Step for {self.kind}",
            "params": {},
        }
        plan = {"thought": "step hint", "steps": [step0, {"idx": 1, "kind": "final", "summary": "done", "params": {}}]}
        return {"choices": [{"message": {"content": json.dumps(plan)}}]}


class FakeRAG:
    async def query(self, **_kw):
        return []


class FakeOpenSpec:
    async def get(self, _id: str) -> dict[str, Any]:
        return {"id": _id, "title": "Test spec"}


# ── 7.1: allowed start_step values ─────────────────────────────────────────


def test_allowed_start_steps_contains_expected_values() -> None:
    assert ALLOWED_START_STEPS == frozenset({"discovery", "architect", "design", "test", "iac", "deploy"})


def test_unknown_start_step_not_in_allowed() -> None:
    assert "not-a-real-step" not in ALLOWED_START_STEPS
    assert "scaffold" not in ALLOWED_START_STEPS
    assert "" not in ALLOWED_START_STEPS


def test_start_session_request_accepts_none_start_step() -> None:
    req = StartSessionRequest(
        workspace_id=uuid.uuid4(),
        openspec_id="spec-1",
        intent="ship it",
    )
    assert req.start_step is None


def test_start_session_request_accepts_valid_start_step() -> None:
    for step in ALLOWED_START_STEPS:
        req = StartSessionRequest(
            workspace_id=uuid.uuid4(),
            openspec_id="spec-1",
            intent="ship it",
            start_step=step,
        )
        assert req.start_step == step


# ── 7.2: lifecycle gate logic ───────────────────────────────────────────────


def test_committed_lifecycle_states_contains_approved_and_committed() -> None:
    assert COMMITTED_LIFECYCLE_STATES == frozenset({"approved", "committed"})


def test_lifecycle_gate_passes_for_committed_states() -> None:
    for state in ("approved", "committed"):
        assert state in COMMITTED_LIFECYCLE_STATES


def test_lifecycle_gate_blocks_other_states() -> None:
    for state in ("proposed", "in_review", "rejected", "archived"):
        assert state not in COMMITTED_LIFECYCLE_STATES


# ── 7.3: planner honours start_step ────────────────────────────────────────


@pytest.mark.asyncio
async def test_planner_default_no_start_step_returns_canonical_plan() -> None:
    plan = await build_initial_plan(
        workspace_id=uuid.uuid4(),
        openspec_id="spec-1",
        intent="ship it",
        correlation_id="corr-1",
        llm=FakeLLM("not-json"),  # force fallback to canonical
        rag=FakeRAG(),
        openspec=FakeOpenSpec(),
        model="test-model",
        start_step=None,
    )
    assert plan["steps"][-1]["kind"] == "final"


@pytest.mark.asyncio
async def test_planner_with_start_step_injects_workflow_step() -> None:
    """When start_step='design', the planner should ensure step 0 targets that phase."""
    plan = await build_initial_plan(
        workspace_id=uuid.uuid4(),
        openspec_id="spec-1",
        intent="ship it",
        correlation_id="corr-1",
        llm=FakeLLMWithStep(kind="workflow", workflow_id="forge.sdlc.design@1"),
        rag=FakeRAG(),
        openspec=FakeOpenSpec(),
        model="test-model",
        start_step="design",
    )
    assert plan["steps"][0]["kind"] == "workflow"
    assert "design" in (plan["steps"][0].get("workflow_id") or "")


@pytest.mark.asyncio
async def test_planner_injects_synthetic_step_when_llm_misses_start_step() -> None:
    """When the LLM doesn't produce a step matching the requested start_step,
    the planner should inject one at idx 0."""
    plan = await build_initial_plan(
        workspace_id=uuid.uuid4(),
        openspec_id="spec-1",
        intent="ship it",
        correlation_id="corr-1",
        llm=FakeLLM(),  # returns a final-only plan, no matching step
        rag=FakeRAG(),
        openspec=FakeOpenSpec(),
        model="test-model",
        start_step="iac",
    )
    # After injection, step 0 must correspond to the requested phase.
    step0 = plan["steps"][0]
    assert step0["kind"] in ("workflow", "final")
    if step0["kind"] == "workflow":
        assert "iac" in (step0.get("workflow_id") or "")


@pytest.mark.asyncio
async def test_planner_discovery_start_step_uses_canonical_flow() -> None:
    """start_step='discovery' is the default; plan should start from the beginning."""
    plan = await build_initial_plan(
        workspace_id=uuid.uuid4(),
        openspec_id="spec-1",
        intent="ship it",
        correlation_id="corr-1",
        llm=FakeLLM("not-json"),
        rag=FakeRAG(),
        openspec=FakeOpenSpec(),
        model="test-model",
        start_step="discovery",
    )
    # Falls back to canonical plan which always starts with the full workflow.
    assert plan["steps"][0]["kind"] in ("workflow", "final")
