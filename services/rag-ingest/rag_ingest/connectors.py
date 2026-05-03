from __future__ import annotations

import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

import httpx

from rag_ingest.models import SourceDocument, SourceKind


class DocumentConnector(Protocol):
    async def fetch(self) -> list[SourceDocument]: ...


@dataclass(frozen=True)
class FilesystemOpenSpecConnector:
    root: Path
    workspace_id: uuid.UUID
    tenant_id: uuid.UUID | None = None

    async def fetch(self) -> list[SourceDocument]:
        documents: list[SourceDocument] = []
        for path in self.root.rglob("*.md"):
            text = path.read_text(encoding="utf-8")
            documents.append(
                SourceDocument(
                    source_kind="filesystem",
                    source_ref=str(path),
                    workspace_id=self.workspace_id,
                    tenant_id=self.tenant_id,
                    title=path.stem,
                    text=text,
                    metadata={"path": str(path)},
                )
            )
        return documents


@dataclass(frozen=True)
class HttpJSONConnector:
    source_kind: SourceKind
    endpoint: str
    workspace_id: uuid.UUID
    tenant_id: uuid.UUID | None = None
    token: str | None = None

    async def fetch(self) -> list[SourceDocument]:
        headers = {"authorization": f"Bearer {self.token}"} if self.token else {}
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.get(self.endpoint, headers=headers)
            response.raise_for_status()
            raw_docs = response.json().get("documents", [])
        return [
            SourceDocument(
                source_kind=self.source_kind,
                source_ref=str(item["source_ref"]),
                workspace_id=self.workspace_id,
                tenant_id=self.tenant_id,
                title=str(item.get("title") or item["source_ref"]),
                text=str(item.get("text") or ""),
                visibility=item.get("visibility", "workspace"),
                data_classification=item.get("data_classification", "internal"),
                provenance_signature=item.get("provenance_signature"),
                metadata=item.get("metadata") or {},
            )
            for item in raw_docs
        ]
