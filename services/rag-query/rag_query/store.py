from __future__ import annotations

import math
import re
from collections import Counter
from dataclasses import dataclass, field

from rag_query.models import IndexedChunk, QueryRequest, QueryResult

TOKEN_RE = re.compile(r"[a-zA-Z0-9_]+")


@dataclass
class InMemoryQueryStore:
    chunks: list[IndexedChunk] = field(default_factory=list)

    def upsert(self, chunks: list[IndexedChunk]) -> None:
        existing = {chunk.chunk_id: chunk for chunk in self.chunks}
        for chunk in chunks:
            existing[chunk.chunk_id] = chunk
        self.chunks = list(existing.values())

    def query(self, request: QueryRequest) -> list[QueryResult]:
        scored: list[QueryResult] = []
        for chunk in self.chunks:
            if not _visible_to_request(chunk, request):
                continue
            score = _score(request.text, chunk.text)
            if score <= 0:
                continue
            scored.append(
                QueryResult(
                    chunk_id=chunk.chunk_id,
                    workspace_id=chunk.workspace_id,
                    tenant_id=chunk.tenant_id,
                    text=chunk.text,
                    score=score,
                    source_ref=chunk.source_ref,
                    source_kind=chunk.source_kind,
                    visibility=chunk.visibility,
                    data_classification=chunk.data_classification,
                    provenance_signed=chunk.provenance_signed,
                    metadata=chunk.metadata,
                )
            )
        scored.sort(key=lambda item: item.score, reverse=True)
        return scored[: request.top_k]


def _visible_to_request(chunk: IndexedChunk, request: QueryRequest) -> bool:
    if not chunk.provenance_signed:
        return False
    if chunk.data_classification not in request.allowed_data_classifications:
        return False
    if chunk.workspace_id == request.workspace_id:
        return True
    return bool(
        chunk.visibility == "tenant"
        and request.tenant_id is not None
        and chunk.tenant_id == request.tenant_id
    )


def _score(query: str, text: str) -> float:
    query_counts = Counter(_tokens(query))
    text_counts = Counter(_tokens(text))
    if not query_counts or not text_counts:
        return 0.0
    dot = sum(query_counts[token] * text_counts[token] for token in query_counts)
    q_norm = math.sqrt(sum(v * v for v in query_counts.values()))
    t_norm = math.sqrt(sum(v * v for v in text_counts.values()))
    return dot / (q_norm * t_norm) if q_norm and t_norm else 0.0


def _tokens(text: str) -> list[str]:
    return [match.group(0).lower() for match in TOKEN_RE.finditer(text)]
