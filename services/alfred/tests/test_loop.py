from __future__ import annotations

import json
import uuid
from typing import Any

import pytest

from alfred.guardrails import Guardrails
from alfred.loop import LoopDeps, run_intent
from alfred.store import InMemoryStore


class FakeLLM:
    def __init__(self, replies: list[dict[str, Any]]) -> None:
        self._replies = replies
        self.calls: list[dict[str, Any]] = []

    async def chat(self, **kwargs):
        self.calls.append(kwargs)
        reply = self._replies.pop(0)
        return {"choices": [{"message": {"content": json.dumps(reply)}}]}


class FakeRAG:
    async def query(self, **_kwargs):
        return [
            {
                "chunk_id": "chunk-1",
                "source_ref": "openspec://phase-1-agentic-core",
                "score": 0.98,
                "text": "Use the OpenSpec as the source of truth.",
            }
        ]


class FakePolicy:
    async def evaluate(self, **_kwargs):
        return {"decision": "allow", "rationale": "golden path"}


class FakeApprovals:
    async def request(self, **_kwargs):
        return {"id": "approval-1", "status": "pending"}


class FakePermissions:
    async def can(self, **_kwargs):
        return {"allowed": True, "reason": "test grant"}


class FakeOpenSpec:
    def __init__(self) -> None:
        self.decisions: list[dict[str, Any]] = []

    async def append_decision(self, _openspec_id: str, decision: dict[str, Any]) -> bool:
        self.decisions.append(decision)
        return True


async def fake_tool(tool_id: str, params: dict[str, Any]) -> dict[str, Any]:
    return {"tool_id": tool_id, "params": params, "token": "secret-token"}


def deps_for(llm: FakeLLM, store: InMemoryStore | None = None) -> LoopDeps:
    return LoopDeps(
        store=store or InMemoryStore(),
        llm=llm,
        rag=FakeRAG(),
        policy=FakePolicy(),
        approvals=FakeApprovals(),
        permissions=FakePermissions(),
        openspec=FakeOpenSpec(),
        guardrails=Guardrails(),
        tool_handler=fake_tool,
        default_model="test-model",
        max_iterations=3,
    )


@pytest.mark.asyncio
async def test_run_intent_records_retrieval_and_final_decision() -> None:
    store = InMemoryStore()
    llm = FakeLLM(
        [
            {
                "thought": "Enough context is available.",
                "next_action": {
                    "kind": "final",
                    "tool_id": None,
                    "params": None,
                    "summary": "OpenSpec draft ready.",
                    "criticality": "low",
                },
            }
        ]
    )
    workspace_id = uuid.uuid4()

    response = await run_intent(
        deps_for(llm, store),
        actor="user-1",
        workspace_id=workspace_id,
        intent="Create an OpenSpec",
        correlation_id="corr-1",
    )

    assert response.final_message == "OpenSpec draft ready."
    decisions = await store.list_decisions(workspace_id=workspace_id)
    assert len(decisions) == 2
    assert decisions[0].correlation_id == "corr-1"
    assert decisions[1].retrieved_refs[0]["source_ref"] == "openspec://phase-1-agentic-core"
    assert llm.calls[0]["metadata"]["correlation_id"] == "corr-1"


@pytest.mark.asyncio
async def test_run_intent_executes_tool_and_redacts_tool_message() -> None:
    store = InMemoryStore()
    llm = FakeLLM(
        [
            {
                "thought": "Call the OpenSpec MCP.",
                "next_action": {
                    "kind": "tool",
                    "tool_id": "mcp:openspec.create",
                    "params": {"title": "Phase 1", "api_token": "secret"},
                    "summary": "create openspec",
                    "criticality": "low",
                },
            },
            {
                "thought": "Tool completed.",
                "next_action": {
                    "kind": "final",
                    "tool_id": None,
                    "params": None,
                    "summary": "Created.",
                    "criticality": "low",
                },
            },
        ]
    )
    workspace_id = uuid.uuid4()

    response = await run_intent(
        deps_for(llm, store),
        actor="user-1",
        workspace_id=workspace_id,
        intent="Create an OpenSpec",
        correlation_id="corr-2",
        openspec_id="OS-1",
    )

    assert response.final_message == "Created."
    messages = await store.list_messages(response.session_id)
    tool_message = next(m for m in messages if m["role"] == "tool")
    assert "***REDACTED***" in tool_message["content"]
    decisions = await store.list_decisions(session_id=response.session_id)
    tool_decision = next(d for d in decisions if d.tool_id == "mcp:openspec.create")
    assert tool_decision.params_redacted["api_token"] == "***REDACTED***"
    assert tool_decision.policy_evaluated == {"decision": "allow", "rationale": "golden path"}
