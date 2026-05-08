"""Models for the incidents knowledge base."""

from __future__ import annotations

from datetime import datetime, timezone

from pydantic import BaseModel, Field


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


class IndexRequest(BaseModel):
    incident_id: str
    tenant_id: str
    workspace_id: str | None = None
    service: str
    environment: str
    summary: str
    symptoms: str
    root_cause: str
    healing_actions: list[str] = Field(default_factory=list)
    resolved_at: datetime | None = None
    synthetic: bool = False


class KBEntry(BaseModel):
    incident_id: str
    tenant_id: str
    workspace_id: str | None = None
    service: str
    environment: str
    summary: str
    symptoms: str
    root_cause: str
    healing_actions: list[str] = Field(default_factory=list)
    indexed_at: datetime = Field(default_factory=utcnow)
    embedding: list[float] = Field(default_factory=list)
    synthetic: bool = False


class SimilarRequest(BaseModel):
    tenant_id: str
    service: str | None = None
    environment: str | None = None
    query: str
    top_k: int = 5
    # The default is intentionally permissive — the diagnosis pipeline filters
    # further by score, and the bag-of-words fallback embedding produces sparse
    # vectors with naturally lower cosine values than transformer embeddings.
    min_score: float = 0.2


class SimilarResult(BaseModel):
    incident_id: str
    score: float
    summary: str
    root_cause: str
    healing_actions: list[str]


class RecurrentCluster(BaseModel):
    cluster_id: str
    tenant_id: str
    incidents: list[str]
    representative_summary: str
    occurrence_count: int
