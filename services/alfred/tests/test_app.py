from __future__ import annotations

import json
import uuid
from typing import Any

from fastapi.testclient import TestClient

from alfred.app import create_app
from alfred.guardrails import Guardrails
from alfred.loop import LoopDeps
from alfred.store import InMemoryStore


class FinalLLM:
    async def chat(self, **_kwargs):
        return {
            "choices": [
                {
                    "message": {
                        "content": json.dumps(
                            {
                                "thought": "No tool needed.",
                                "next_action": {
                                    "kind": "final",
                                    "tool_id": None,
                                    "params": None,
                                    "summary": "Ready.",
                                    "criticality": "low",
                                },
                            }
                        )
                    }
                }
            ]
        }


class EmptyRAG:
    async def query(self, **_kwargs):
        return []


class AllowPolicy:
    async def evaluate(self, **_kwargs):
        return {"decision": "allow", "rationale": "test"}


class NoopApprovals:
    async def request(self, **_kwargs):
        return {"id": "approval-1"}


class AllowPermissions:
    async def can(self, **_kwargs):
        return {"allowed": True}


class NoopOpenSpec:
    async def append_decision(self, *_args, **_kwargs):
        return True


async def fake_tool(tool_id: str, params: dict[str, Any]) -> dict[str, Any]:
    return {"tool_id": tool_id, "params": params}


def test_app_exposes_intent_session_message_and_decision_endpoints() -> None:
    store = InMemoryStore()
    deps = LoopDeps(
        store=store,
        llm=FinalLLM(),
        rag=EmptyRAG(),
        policy=AllowPolicy(),
        approvals=NoopApprovals(),
        permissions=AllowPermissions(),
        openspec=NoopOpenSpec(),
        guardrails=Guardrails(),
        tool_handler=fake_tool,
        default_model="test-model",
    )
    app = create_app(store=store, loop_deps=deps, auth_required=False)
    workspace_id = str(uuid.uuid4())

    with TestClient(app) as client:
        intent = client.post(
            "/v1/intents",
            json={"workspace_id": workspace_id, "text": "Draft an OpenSpec"},
            headers={"X-Correlation-Id": "corr-api"},
        )
        assert intent.status_code == 200
        assert intent.headers["X-Correlation-Id"] == "corr-api"
        body = intent.json()
        assert body["final_message"] == "Ready."

        session = client.get(f"/v1/sessions/{body['session_id']}")
        assert session.status_code == 200
        assert session.json()["session"]["correlation_id"] == "corr-api"

        message = client.post(
            f"/v1/sessions/{body['session_id']}/messages",
            json={"role": "user", "content": "follow-up"},
        )
        assert message.status_code == 200

        decisions = client.get("/v1/decisions", params={"session_id": body["session_id"]})
        assert decisions.status_code == 200
        assert len(decisions.json()["decisions"]) == 2
