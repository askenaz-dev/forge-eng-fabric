from __future__ import annotations

import uuid
from typing import Any, Literal

from pydantic import BaseModel, Field

DataClassification = Literal["public", "internal", "confidential", "restricted"]
Visibility = Literal["workspace", "tenant"]


class QueryRequest(BaseModel):
    workspace_id: uuid.UUID
    text: str
    principal: str
    top_k: int = Field(default=8, ge=1, le=50)
    tenant_id: uuid.UUID | None = None
    allowed_data_classifications: list[DataClassification] = Field(default_factory=lambda: ["public", "internal"])


class QueryResult(BaseModel):
    chunk_id: str
    workspace_id: uuid.UUID
    tenant_id: uuid.UUID | None = None
    text: str
    score: float
    source_ref: str
    source_kind: str
    visibility: Visibility
    data_classification: DataClassification
    provenance_signed: bool
    metadata: dict[str, Any] = Field(default_factory=dict)


class QueryResponse(BaseModel):
    results: list[QueryResult]


class IndexedChunk(BaseModel):
    chunk_id: str
    workspace_id: uuid.UUID
    tenant_id: uuid.UUID | None = None
    text: str
    source_ref: str
    source_kind: str
    visibility: Visibility = "workspace"
    data_classification: DataClassification = "internal"
    provenance_signed: bool = True
    metadata: dict[str, Any] = Field(default_factory=dict)
