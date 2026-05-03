from __future__ import annotations

import time
import uuid
from dataclasses import dataclass, field
from typing import Any, Protocol

import httpx

from alfred.redaction import redact


class AIObserver(Protocol):
    async def capture_model_call(
        self,
        *,
        model: str,
        messages: list[dict[str, Any]],
        response: dict[str, Any] | None,
        metadata: dict[str, Any],
        latency_ms: float,
        error: str | None = None,
    ) -> None: ...

    async def capture_tool_call(
        self,
        *,
        tool_id: str,
        params: dict[str, Any],
        result: dict[str, Any] | None,
        metadata: dict[str, Any],
        latency_ms: float,
        error: str | None = None,
    ) -> None: ...


@dataclass
class InMemoryAIObserver:
    traces: list[dict[str, Any]] = field(default_factory=list)

    async def capture_model_call(
        self,
        *,
        model: str,
        messages: list[dict[str, Any]],
        response: dict[str, Any] | None,
        metadata: dict[str, Any],
        latency_ms: float,
        error: str | None = None,
    ) -> None:
        self.traces.append(model_call_event(model, messages, response, metadata, latency_ms, error))

    async def capture_tool_call(
        self,
        *,
        tool_id: str,
        params: dict[str, Any],
        result: dict[str, Any] | None,
        metadata: dict[str, Any],
        latency_ms: float,
        error: str | None = None,
    ) -> None:
        self.traces.append(tool_call_event(tool_id, params, result, metadata, latency_ms, error))


class LangfuseObserver:
    def __init__(self, *, host: str, public_key: str, secret_key: str) -> None:
        self._host = host.rstrip("/")
        self._auth = (public_key, secret_key)

    async def capture_model_call(
        self,
        *,
        model: str,
        messages: list[dict[str, Any]],
        response: dict[str, Any] | None,
        metadata: dict[str, Any],
        latency_ms: float,
        error: str | None = None,
    ) -> None:
        event = model_call_event(model, messages, response, metadata, latency_ms, error)
        async with httpx.AsyncClient(timeout=5.0) as client:
            await client.post(f"{self._host}/api/public/ingestion", auth=self._auth, json={"batch": [event]})

    async def capture_tool_call(
        self,
        *,
        tool_id: str,
        params: dict[str, Any],
        result: dict[str, Any] | None,
        metadata: dict[str, Any],
        latency_ms: float,
        error: str | None = None,
    ) -> None:
        event = tool_call_event(tool_id, params, result, metadata, latency_ms, error)
        async with httpx.AsyncClient(timeout=5.0) as client:
            await client.post(f"{self._host}/api/public/ingestion", auth=self._auth, json={"batch": [event]})


def model_call_event(
    model: str,
    messages: list[dict[str, Any]],
    response: dict[str, Any] | None,
    metadata: dict[str, Any],
    latency_ms: float,
    error: str | None,
) -> dict[str, Any]:
    classification = str(metadata.get("data_classification") or "internal")
    trace_id = str(metadata.get("langfuse_trace_id") or metadata.get("correlation_id") or uuid.uuid4())
    return {
        "id": str(uuid.uuid4()),
        "type": "generation-create",
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "body": {
            "id": str(uuid.uuid4()),
            "traceId": trace_id,
            "name": "litellm.chat",
            "model": model,
            "input": redact_by_classification(messages, classification),
            "output": redact_by_classification(response or {}, classification),
            "metadata": redact(metadata),
            "level": "ERROR" if error else "DEFAULT",
            "statusMessage": error,
            "usage": _usage(response or {}),
            "latency_ms": latency_ms,
        },
    }


def tool_call_event(
    tool_id: str,
    params: dict[str, Any],
    result: dict[str, Any] | None,
    metadata: dict[str, Any],
    latency_ms: float,
    error: str | None,
) -> dict[str, Any]:
    classification = str(metadata.get("data_classification") or "internal")
    trace_id = str(metadata.get("langfuse_trace_id") or metadata.get("correlation_id") or uuid.uuid4())
    return {
        "id": str(uuid.uuid4()),
        "type": "span-create",
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "body": {
            "id": str(uuid.uuid4()),
            "traceId": trace_id,
            "name": f"tool.{tool_id}",
            "input": redact_by_classification(params, classification),
            "output": redact_by_classification(result or {}, classification),
            "metadata": redact({**metadata, "tool_id": tool_id}),
            "level": "ERROR" if error else "DEFAULT",
            "statusMessage": error,
            "latency_ms": latency_ms,
        },
    }


def redact_by_classification(value: Any, classification: str) -> Any:
    redacted = redact(value)
    if classification in {"confidential", "restricted"}:
        return "***REDACTED_BY_CLASSIFICATION***"
    return redacted


def _usage(response: dict[str, Any]) -> dict[str, Any]:
    usage = response.get("usage") or {}
    return {
        "input": usage.get("prompt_tokens", 0),
        "output": usage.get("completion_tokens", 0),
        "total": usage.get("total_tokens", 0),
        "unit": "TOKENS",
    }
