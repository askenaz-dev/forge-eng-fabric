from __future__ import annotations

import re
import uuid
from dataclasses import dataclass, field
from typing import Protocol

from rag_ingest.models import ChunkRecord


class VectorSink(Protocol):
    async def write(self, chunks: list[ChunkRecord]) -> None: ...


def collection_name_for(*, workspace_id: uuid.UUID | None, visibility: str) -> str:
    if visibility == "tenant":
        return "forge_rag_tenant_shared"
    if not workspace_id:
        raise ValueError("workspace_id is required for workspace-visible chunks")
    safe = re.sub(r"[^a-zA-Z0-9_]", "_", str(workspace_id))
    return f"forge_rag_ws_{safe}"


@dataclass
class InMemoryVectorSink:
    chunks: list[ChunkRecord] = field(default_factory=list)

    async def write(self, chunks: list[ChunkRecord]) -> None:
        self.chunks.extend(chunks)


class MilvusVectorSink:
    """Milvus writer with per-workspace collections and tenant-shared collection.

    `pymilvus` is imported lazily so unit tests and local docs can run without a
    Milvus client installed. The row schema is intentionally flat to keep query
    filters simple: workspace_id, tenant_id, classification, source_ref and
    provenance_signed are stored as scalar fields alongside the vector.
    """

    def __init__(self, uri: str, token: str | None = None) -> None:
        self._uri = uri
        self._token = token

    async def write(self, chunks: list[ChunkRecord]) -> None:
        from pymilvus import MilvusClient  # type: ignore[import-not-found]

        client = MilvusClient(uri=self._uri, token=self._token)
        by_collection: dict[str, list[dict[str, object]]] = {}
        for chunk in chunks:
            by_collection.setdefault(chunk.collection_name, []).append(_chunk_to_row(chunk))
        for collection, rows in by_collection.items():
            if not client.has_collection(collection):
                client.create_collection(collection_name=collection, dimension=len(chunks[0].embedding))
            client.insert(collection_name=collection, data=rows)


def _chunk_to_row(chunk: ChunkRecord) -> dict[str, object]:
    return {
        "id": chunk.chunk_id,
        "vector": chunk.embedding,
        "text": chunk.text,
        "workspace_id": str(chunk.workspace_id),
        "tenant_id": str(chunk.tenant_id) if chunk.tenant_id else "",
        "visibility": chunk.visibility,
        "data_classification": chunk.data_classification,
        "source_kind": chunk.source_kind,
        "source_ref": chunk.source_ref,
        "provenance_signed": chunk.provenance_signed,
        "metadata": chunk.metadata,
    }
