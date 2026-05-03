from __future__ import annotations

import hashlib
from typing import Protocol

import httpx


class EmbeddingsClient(Protocol):
    async def embed(self, texts: list[str]) -> list[list[float]]: ...


class LiteLLMEmbeddings:
    def __init__(self, *, base_url: str, api_key: str, model: str) -> None:
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._model = model

    async def embed(self, texts: list[str]) -> list[list[float]]:
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.post(
                f"{self._base_url}/v1/embeddings",
                headers={"authorization": f"Bearer {self._api_key}"},
                json={"model": self._model, "input": texts},
            )
            response.raise_for_status()
            rows = response.json().get("data", [])
        return [row["embedding"] for row in rows]


class DeterministicEmbeddings:
    """Small deterministic embedder for tests and local demos without an LLM key."""

    def __init__(self, dimensions: int = 16) -> None:
        self._dimensions = dimensions

    async def embed(self, texts: list[str]) -> list[list[float]]:
        vectors: list[list[float]] = []
        for text in texts:
            digest = hashlib.sha256(text.encode("utf-8")).digest()
            vectors.append([digest[i % len(digest)] / 255 for i in range(self._dimensions)])
        return vectors
