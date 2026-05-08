"""Data models for the diagnosis pipeline."""

from __future__ import annotations

from datetime import datetime, timezone

from pydantic import BaseModel, Field


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


class DiagnosisRequest(BaseModel):
    incident_id: str
    tenant_id: str
    workspace_id: str | None = None
    service: str
    environment: str
    signature_hash: str
    severity: str
    title: str
    description: str | None = None
    synthetic: bool = False


class Citation(BaseModel):
    """Anchors a piece of evidence to a verifiable source."""

    source_kind: str  # runbook | openspec | metric | log | trace | kb_incident | eval | finops
    source_id: str
    url: str | None = None
    excerpt: str | None = None
    score: float | None = None


class EvidenceBlock(BaseModel):
    kind: str
    summary: str
    citations: list[Citation] = Field(default_factory=list)
    raw: dict | None = None


class ContextBundle(BaseModel):
    request: DiagnosisRequest
    evidence: list[EvidenceBlock] = Field(default_factory=list)
    gathered_at: datetime = Field(default_factory=utcnow)


class Hypothesis(BaseModel):
    statement: str
    confidence: float
    citations: list[Citation]
    suggested_actions: list[str] = Field(default_factory=list)
    rationale: str | None = None


class DiagnosisReport(BaseModel):
    incident_id: str
    prompt_version: str
    model: str
    hypotheses: list[Hypothesis]
    context_summary: str
    duration_ms: float
    started_at: datetime
    finished_at: datetime
    synthetic: bool = False
