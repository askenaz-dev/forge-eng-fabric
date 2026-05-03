from __future__ import annotations

from alfred.observability import (
    InMemoryAIObserver,
    model_call_event,
    redact_by_classification,
    tool_call_event,
)


def test_redaction_by_classification_and_correlation_trace() -> None:
    event = model_call_event(
        "test-model",
        messages=[{"role": "user", "content": "secret", "api_token": "abc"}],
        response={"choices": [], "usage": {"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3}},
        metadata={"correlation_id": "corr-ai", "data_classification": "internal"},
        latency_ms=12.5,
        error=None,
    )
    body = event["body"]
    assert body["traceId"] == "corr-ai"
    assert body["input"][0]["api_token"] == "***REDACTED***"
    assert body["usage"]["total"] == 3
    assert redact_by_classification({"content": "do not show"}, "restricted") == "***REDACTED_BY_CLASSIFICATION***"


def test_tool_call_event_shares_correlation_trace_and_redacts() -> None:
    event = tool_call_event(
        "skill:generate-test-cases",
        params={"api_token": "abc", "openspec_id": "OS-1"},
        result={"test_cases": [{"id": "TC-001"}]},
        metadata={"correlation_id": "corr-tool"},
        latency_ms=4.2,
        error=None,
    )
    body = event["body"]
    assert event["type"] == "span-create"
    assert body["traceId"] == "corr-tool"
    assert body["input"]["api_token"] == "***REDACTED***"
    assert body["metadata"]["tool_id"] == "skill:generate-test-cases"


async def test_in_memory_ai_observer_records_model_calls() -> None:
    observer = InMemoryAIObserver()
    await observer.capture_model_call(
        model="test",
        messages=[],
        response={"usage": {}},
        metadata={"correlation_id": "corr-memory"},
        latency_ms=1,
    )
    assert observer.traces[0]["body"]["traceId"] == "corr-memory"

    await observer.capture_tool_call(
        tool_id="mcp:openspec.create",
        params={},
        result={"ok": True},
        metadata={"correlation_id": "corr-memory"},
        latency_ms=1,
    )
    assert observer.traces[1]["body"]["name"] == "tool.mcp:openspec.create"
