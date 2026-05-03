from __future__ import annotations

import json
import uuid
from datetime import datetime, timedelta
from typing import Any

import jsonschema
import pytest

from alfred.guardrails import Guardrails
from alfred.loop import LoopDeps, run_intent
from alfred.observability import InMemoryAIObserver
from alfred.store import InMemoryStore

USER_STORIES_SCHEMA = {
    "type": "object",
    "required": ["epics", "stories"],
    "properties": {
        "epics": {"type": "array", "minItems": 1},
        "stories": {"type": "array", "minItems": 1},
    },
}

TEST_CASES_SCHEMA = {
    "type": "object",
    "required": ["test_cases"],
    "properties": {"test_cases": {"type": "array", "minItems": 1}},
}

CRITICALITY_RANK = {"low": 1, "medium": 2, "high": 3, "critical": 4}


class ScriptedLLM:
    def __init__(self, replies: list[dict[str, Any]], observer: InMemoryAIObserver) -> None:
        self._replies = replies
        self._observer = observer

    async def chat(self, *, model: str, messages: list[dict[str, Any]], metadata: dict[str, Any] | None = None, **_kwargs):
        reply = self._replies.pop(0)
        response = {
            "choices": [{"message": {"content": json.dumps(reply)}}],
            "usage": {"prompt_tokens": 8, "completion_tokens": 5, "total_tokens": 13},
        }
        await self._observer.capture_model_call(
            model=model,
            messages=messages,
            response=response,
            metadata=metadata or {},
            latency_ms=1,
        )
        return response


class LocalRAG:
    async def query(self, **_kwargs):
        return [
            {
                "chunk_id": "phase-1-e2e",
                "source_ref": "openspec://phase-1-agentic-core",
                "score": 0.99,
                "provenance_signed": True,
                "text": "Phase 1 exit requires OpenSpec, permissions, policy, skills, lifecycle and audit.",
            }
        ]


class LocalPermissions:
    def __init__(self) -> None:
        self.grants: list[dict[str, Any]] = []

    def grant(
        self,
        *,
        subject: str,
        action_class: str,
        scope_kind: str,
        scope_id: str,
        max_criticality: str,
        expiration_days: int,
    ) -> None:
        created_at = datetime.utcnow()
        self.grants.append(
            {
                "subject": subject,
                "action_class": action_class,
                "scope_kind": scope_kind,
                "scope_id": scope_id,
                "max_criticality": max_criticality,
                "expiration_days": expiration_days,
                "created_at": created_at,
                "expires_at": created_at + timedelta(days=expiration_days),
                "status": "active",
            }
        )

    async def can(
        self,
        *,
        subject: str,
        action_class: str,
        scope_kind: str,
        scope_id: str,
        criticality: str,
    ) -> dict[str, Any]:
        now = datetime.utcnow()
        for grant in self.grants:
            if (
                grant["status"] == "active"
                and grant["subject"] == subject
                and grant["action_class"] == action_class
                and grant["scope_kind"] == scope_kind
                and grant["scope_id"] == scope_id
                and grant["expires_at"] > now
                and CRITICALITY_RANK[criticality] <= CRITICALITY_RANK[grant["max_criticality"]]
            ):
                return {"allowed": True, "reason": "active delegated permission"}
        return {"allowed": False, "reason": "no active delegated permission"}


class LocalPolicy:
    async def evaluate(self, *, action: str, env: str = "dev", **_kwargs) -> dict[str, Any]:
        if action == "deploy:prod" or env == "prod":
            return {
                "decision": "requires_approval",
                "rationale": "production-impacting actions require approval",
                "policy_id": "deploy-prod-requires-approval",
                "required_approvers": ["release-manager"],
            }
        return {"decision": "allow", "rationale": "workspace default autonomous", "policy_id": "default-allow"}


class LocalApprovals:
    def __init__(self) -> None:
        self.requests: list[dict[str, Any]] = []

    async def request(self, **kwargs):
        self.requests.append(kwargs)
        return {"id": "approval-phase-1", "status": "pending"}


class OpenSpecAudit:
    def __init__(self) -> None:
        self.decisions: list[dict[str, Any]] = []

    async def append_decision(self, openspec_id: str, decision: dict[str, Any]) -> bool:
        self.decisions.append({"openspec_id": openspec_id, **decision})
        return True


class LocalToolRunner:
    def __init__(self) -> None:
        self.created_openspec: dict[str, Any] | None = None
        self.validated_outputs: list[str] = []

    async def __call__(self, tool_id: str, params: dict[str, Any]) -> dict[str, Any]:
        if tool_id == "mcp:openspec.create":
            artifacts = params.get("linked_artifacts") or []
            kinds = {artifact.get("kind") for artifact in artifacts}
            if not {"jira", "confluence"}.issubset(kinds):
                raise ValueError("OpenSpec must link Jira and Confluence artifacts")
            self.created_openspec = params
            return {"openspec_id": params["openspec_id"], "linked_artifacts": artifacts}
        if tool_id == "skill:create-user-stories":
            output = {
                "epics": [{"key": "EPIC-PHASE1", "summary": "Phase 1 E2E", "openspec_id": "phase-1-e2e"}],
                "stories": [
                    {
                        "key": "STORY-PHASE1",
                        "summary": "Validate Phase 1 exit criteria",
                        "epic_key": "EPIC-PHASE1",
                        "links": {"openspec_id": "phase-1-e2e", "direction": "bidirectional"},
                    }
                ],
            }
            jsonschema.validate(output, USER_STORIES_SCHEMA)
            self.validated_outputs.append(tool_id)
            return output
        if tool_id == "skill:generate-test-cases":
            output = {
                "test_cases": [
                    {
                        "id": "TC-001",
                        "title": "Validate Phase 1 audit trail",
                        "type": "acceptance",
                        "steps": ["Submit intent", "Invoke tools", "Inspect decisions"],
                        "expected": "Every decision shares the intent correlation_id",
                    }
                ]
            }
            jsonschema.validate(output, TEST_CASES_SCHEMA)
            self.validated_outputs.append(tool_id)
            return output
        raise ValueError(f"unsupported tool {tool_id}")


@pytest.mark.asyncio
async def test_phase1_local_e2e_exit_criteria_smoke() -> None:
    workspace_id = uuid.uuid4()
    correlation_id = "phase-1-e2e-correlation"
    observer = InMemoryAIObserver()
    store = InMemoryStore()
    policy = LocalPolicy()
    permissions = LocalPermissions()
    tools = LocalToolRunner()
    openspec_audit = OpenSpecAudit()

    permissions.grant(
        subject="alfred",
        action_class="openspec:write",
        scope_kind="workspace",
        scope_id=str(workspace_id),
        max_criticality="medium",
        expiration_days=7,
    )
    permissions.grant(
        subject="alfred",
        action_class="skill:invoke",
        scope_kind="workspace",
        scope_id=str(workspace_id),
        max_criticality="medium",
        expiration_days=7,
    )

    assert (await policy.evaluate(action="deploy:prod", env="prod"))["decision"] == "requires_approval"
    assert (await policy.evaluate(action="mcp:openspec.create", env="dev"))["decision"] == "allow"
    assert next(grant for grant in permissions.grants if grant["action_class"] == "openspec:write")["expiration_days"] == 7

    llm = ScriptedLLM(
        [
            {
                "thought": "Create the living contract and link external planning/docs artifacts.",
                "next_action": {
                    "kind": "tool",
                    "tool_id": "mcp:openspec.create",
                    "params": {
                        "openspec_id": "phase-1-e2e",
                        "workspace_id": str(workspace_id),
                        "title": "Phase 1 E2E",
                        "business_intent": "Validate first agentic value path",
                        "requirements": {"functional": ["Create Jira stories", "Generate acceptance tests"]},
                        "linked_artifacts": [
                            {"kind": "jira", "ref": "JIRA-123", "direction": "bidirectional"},
                            {"kind": "confluence", "ref": "https://confluence.example/phase-1", "direction": "bidirectional"},
                        ],
                    },
                    "summary": "create OpenSpec",
                    "criticality": "low",
                },
            },
            {
                "thought": "Generate Jira stories from the OpenSpec.",
                "next_action": {
                    "kind": "tool",
                    "tool_id": "skill:create-user-stories",
                    "params": {
                        "openspec": {
                            "openspec_id": "phase-1-e2e",
                            "title": "Phase 1 E2E",
                            "requirements": {"functional": ["Create Jira stories"]},
                        }
                    },
                    "summary": "generate stories",
                    "criticality": "low",
                },
            },
            {
                "thought": "Generate acceptance tests from the same OpenSpec.",
                "next_action": {
                    "kind": "tool",
                    "tool_id": "skill:generate-test-cases",
                    "params": {
                        "openspec": {
                            "openspec_id": "phase-1-e2e",
                            "title": "Phase 1 E2E",
                            "requirements": {"functional": ["Generate acceptance tests"]},
                        }
                    },
                    "summary": "generate tests",
                    "criticality": "low",
                },
            },
            {
                "thought": "All local E2E evidence was collected.",
                "next_action": {
                    "kind": "final",
                    "tool_id": None,
                    "params": None,
                    "summary": "Phase 1 local E2E smoke completed.",
                    "criticality": "low",
                },
            },
        ],
        observer,
    )
    deps = LoopDeps(
        store=store,
        llm=llm,
        rag=LocalRAG(),
        policy=policy,
        approvals=LocalApprovals(),
        permissions=permissions,
        openspec=openspec_audit,
        guardrails=Guardrails(),
        tool_handler=tools,
        default_model="test-model",
        max_iterations=5,
        ai_observer=observer,
    )

    response = await run_intent(
        deps,
        actor="alice",
        workspace_id=workspace_id,
        intent="Validate Phase 1 E2E from Alfred Console",
        correlation_id=correlation_id,
        openspec_id="phase-1-e2e",
        metadata={"env": "dev"},
    )

    assert response.final_message == "Phase 1 local E2E smoke completed."
    assert tools.created_openspec is not None
    assert {artifact["kind"] for artifact in tools.created_openspec["linked_artifacts"]} == {"jira", "confluence"}
    assert tools.validated_outputs == ["skill:create-user-stories", "skill:generate-test-cases"]

    decisions = await store.list_decisions(correlation_id=correlation_id)
    tool_decisions = {decision.tool_id: decision for decision in decisions if decision.tool_id}
    for tool_id in ["mcp:openspec.create", "skill:create-user-stories", "skill:generate-test-cases"]:
        assert tool_decisions[tool_id].outcome == "succeeded"
        assert tool_decisions[tool_id].correlation_id == correlation_id
    assert all(decision["correlation_id"] == correlation_id for decision in openspec_audit.decisions)

    trace_names = [trace["body"]["name"] for trace in observer.traces]
    assert "litellm.chat" in trace_names
    assert "tool.mcp:openspec.create" in trace_names
    assert "tool.skill:create-user-stories" in trace_names
    assert "tool.skill:generate-test-cases" in trace_names
    assert all(trace["body"]["traceId"] == correlation_id for trace in observer.traces)
