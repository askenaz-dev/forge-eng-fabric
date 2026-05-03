"""LiteLLM client. Alfred MUST NOT call providers directly — only via this gateway."""

from __future__ import annotations

import time
from typing import Any

import httpx
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_exponential

from alfred.observability import AIObserver


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
        tools: list[dict[str, Any]] | None = None,
        metadata: dict[str, Any] | None = None,
        max_tokens: int | None = None,
    ) -> dict[str, Any]:
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
                    headers={"authorization": f"Bearer {self._key}"},
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

    async def embed(self, *, model: str, inputs: list[str]) -> list[list[float]]:
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            r = await client.post(
                f"{self._base}/v1/embeddings",
                headers={"authorization": f"Bearer {self._key}"},
                json={"model": model, "input": inputs},
            )
            r.raise_for_status()
            data = r.json()
            return [row["embedding"] for row in data.get("data", [])]
