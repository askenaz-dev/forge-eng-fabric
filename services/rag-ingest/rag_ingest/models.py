from __future__ import annotations

import uuid
from typing import Any, Literal

from pydantic import BaseModel, Field

SourceKind = Literal["filesystem", "github", "confluence", "jira", "registry", "audit", "incident"]
Visibility = Literal["workspace", "tenant"]
DataClassification = Literal["public", "internal", "confidential", "restricted"]


class SourceDocument(BaseModel):
    source_kind: SourceKind
    source_ref: str
    workspace_id: uuid.UUID
    tenant_id: uuid.UUID | None = None
    title: str
    text: str
    visibility: Visibility = "workspace"
    data_classification: DataClassification = "internal"
    provenance_signature: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class ChunkRecord(BaseModel):
    chunk_id: str
    workspace_id: uuid.UUID
    tenant_id: uuid.UUID | None = None
    collection_name: str
    text: str
    embedding: list[float]
    visibility: Visibility
    data_classification: DataClassification
    source_kind: SourceKind
    source_ref: str
    provenance_signed: bool
    metadata: dict[str, Any] = Field(default_factory=dict)


class IngestRequest(BaseModel):
    documents: list[SourceDocument]
    require_signed_sources: bool = True


class IngestResponse(BaseModel):
    accepted_documents: int
    rejected_documents: int
    chunks_written: int
    rejected: list[dict[str, str]] = Field(default_factory=list)
