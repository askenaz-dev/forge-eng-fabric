"""LiteLLM client. Alfred MUST NOT call providers directly — only via this gateway."""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any

import httpx
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_exponential

from alfred.observability import AIObserver


class LiteLLMHeaderError(ValueError):
    """Raised when a LiteLLM call is missing one of the required tenant headers.

    alfred-litellm-header-injection (G1) makes Alfred fail closed rather than
    issue an unattributed request. The four standard headers
    (`forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, `data_classification`)
    are required on every chat/embeddings request; the first three must be
    populated by the caller, the last defaults to `internal`.
    """


_VALID_CLASSIFICATIONS = frozenset({"public", "internal", "confidential", "restricted"})


@dataclass(frozen=True)
class RequestContext:
    """The four pieces of context Alfred MUST attach to every LiteLLM request.

    Build one per outbound call from the active session. The reasoning loop
    and agent-mode pass it as an explicit argument (design D2) — no
    contextvars magic — so the dependency is visible at every callsite.
    """

    tenant_id: str
    workspace_id: str
    correlation_id: str
    data_classification: str = "internal"

    @classmethod
    def system(cls, *, correlation_id: str) -> "RequestContext":
        """Factory for system-internal callsites without a session (e.g. startup
        health checks). Uses sentinel tenant/workspace ids and an `internal`
        classification. Callers should prefer building from an active session
        when one exists.
        """

        return cls(
            tenant_id="system",
            workspace_id="system",
            correlation_id=correlation_id,
            data_classification="internal",
        )

    def headers(self) -> dict[str, str]:
        """Return the four standard headers. Raises `LiteLLMHeaderError` if
        any of the required fields is empty/null. `data_classification` is
        validated against the spec's enum; unknown values raise.
        """

        if not self.tenant_id:
            raise LiteLLMHeaderError("RequestContext.tenant_id is required")
        if not self.workspace_id:
            raise LiteLLMHeaderError("RequestContext.workspace_id is required")
        if not self.correlation_id:
            raise LiteLLMHeaderError("RequestContext.correlation_id is required")
        classification = self.data_classification or "internal"
        if classification not in _VALID_CLASSIFICATIONS:
            raise LiteLLMHeaderError(
                f"data_classification must be one of {sorted(_VALID_CLASSIFICATIONS)}, "
                f"got {classification!r}"
            )
        return {
            "forgetenantid": self.tenant_id,
            "forgeworkspaceid": self.workspace_id,
            "forgecorrelationid": self.correlation_id,
            "data_classification": classification,
        }


class LiteLLMClient:
    def __init__(self, base_url: str, api_key: str, timeout: float = 60.0, observer: AIObserver | None = None) -> None:
        self._base = base_url.rstrip("/")
        self._key = api_key
        self._timeout = timeout
        self._observer = observer

    @retry(
        reraise=True,
        retry=retry_if_exception_type((httpx.TransportError, httpx.RemoteProtocolError)),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=0.5, max=4),
    )
    async def chat(
        self,
        *,
        model: str,
        messages: list[dict[str, Any]],
        context: RequestContext,
        tools: list[dict[str, Any]] | None = None,
        metadata: dict[str, Any] | None = None,
        max_tokens: int | None = None,
    ) -> dict[str, Any]:
        headers = {"authorization": f"Bearer {self._key}", **context.headers()}
        payload: dict[str, Any] = {"model": model, "messages": messages}
        if tools:
            payload["tools"] = tools
        if max_tokens:
            payload["max_tokens"] = max_tokens
        if metadata:
            payload["metadata"] = metadata
        started = time.perf_counter()
        try:
            async with httpx.AsyncClient(timeout=self._timeout) as client:
                r = await client.post(
                    f"{self._base}/v1/chat/completions",
                    headers=headers,
                    json=payload,
                )
                r.raise_for_status()
                response = r.json()
        except Exception as exc:
            if self._observer:
                await self._observer.capture_model_call(
                    model=model,
                    messages=messages,
                    response=None,
                    metadata=metadata or {},
                    latency_ms=(time.perf_counter() - started) * 1000,
                    error=str(exc),
                )
            raise
        if self._observer:
            await self._observer.capture_model_call(
                model=model,
                messages=messages,
                response=response,
                metadata=metadata or {},
                latency_ms=(time.perf_counter() - started) * 1000,
            )
        return response

    async def embed(
        self,
        *,
        model: str,
        inputs: list[str],
        context: RequestContext,
    ) -> list[list[float]]:
        headers = {"authorization": f"Bearer {self._key}", **context.headers()}
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            r = await client.post(
                f"{self._base}/v1/embeddings",
                headers=headers,
                json={"model": model, "input": inputs},
            )
            r.raise_for_status()
            data = r.json()
            return [row["embedding"] for row in data.get("data", [])]
