from __future__ import annotations

import json
import uuid
from typing import Any

import pytest

from alfred.agent_mode.planner import CANONICAL_PLAN, build_initial_plan, replan


class FakeLLM:
    def __init__(self, content: str) -> None:
        self.content = content
        self.calls: list[dict[str, Any]] = []

    async def chat(self, **kwargs):
        self.calls.append(kwargs)
        return {"choices": [{"message": {"content": self.content}}]}


class FakeRAG:
    async def query(self, **_kwargs):
        return [{"source_ref": "openspec://x", "text": "context"}]


class FakeOpenSpec:
    async def get(self, _id):
        return {"id": _id, "title": "demo"}


@pytest.mark.asyncio
async def test_planner_returns_canonical_plan_on_garbage_output() -> None:
    plan = await build_initial_plan(
        workspace_id=uuid.uuid4(),
        openspec_id="spec-1",
        intent="ship a hello-world service",
        correlation_id="corr-1",
        llm=FakeLLM("not-json at all"),
        rag=FakeRAG(),
        openspec=FakeOpenSpec(),
        model="test-model",
    )
    assert plan["steps"]
    # Canonical plan always ends with a `final` step.
    assert plan["steps"][-1]["kind"] == "final"


@pytest.mark.asyncio
async def test_planner_accepts_valid_llm_plan() -> None:
    valid = {
        "thought": "plan generated",
        "steps": [
            {
                "idx": 0,
                "kind": "workflow",
                "workflow_id": "forge.reference.intent-to-deploy@1",
                "criticality": "high",
                "summary": "Run reference workflow",
                "params": {},
            },
            {"idx": 1, "kind": "final", "summary": "report", "params": {}},
        ],
    }
    plan = await build_initial_plan(
        workspace_id=uuid.uuid4(),
        openspec_id="spec-1",
        intent="x",
        correlation_id="c",
        llm=FakeLLM(json.dumps(valid)),
        rag=FakeRAG(),
        openspec=FakeOpenSpec(),
        model="test-model",
    )
    assert plan["steps"][0]["kind"] == "workflow"
    assert plan["steps"][0]["workflow_id"] == "forge.reference.intent-to-deploy@1"


@pytest.mark.asyncio
async def test_planner_strips_code_fences() -> None:
    valid = {
        "thought": "plan",
        "steps": [{"idx": 0, "kind": "final", "summary": "done", "params": {}}],
    }
    fenced = "```json\n" + json.dumps(valid) + "\n```"
    plan = await build_initial_plan(
        workspace_id=uuid.uuid4(),
        openspec_id="s",
        intent="x",
        correlation_id="c",
        llm=FakeLLM(fenced),
        rag=FakeRAG(),
        openspec=FakeOpenSpec(),
        model="m",
    )
    assert plan["steps"][0]["kind"] == "final"


def test_replan_inserts_fix_step_and_renumbers() -> None:
    plan = {
        "thought": "initial",
        "steps": [
            {"idx": 0, "kind": "tool", "tool_id": "skill:a", "summary": "first"},
            {"idx": 1, "kind": "tool", "tool_id": "skill:b", "summary": "second"},
            {"idx": 2, "kind": "final", "summary": "done"},
        ],
    }
    new_plan = replan(plan, failed_step_idx=0, reason="boom")
    # Original step zero is preserved, fix is inserted at idx 1, rest pushed down.
    assert [s["idx"] for s in new_plan["steps"]] == [0, 1, 2, 3]
    assert new_plan["steps"][1]["tool_id"] == "skill:diagnose-and-retry"
    assert "boom" in new_plan["thought"]


def test_canonical_plan_is_self_consistent() -> None:
    assert CANONICAL_PLAN["steps"][0]["kind"] == "workflow"
    assert CANONICAL_PLAN["steps"][-1]["kind"] == "final"
