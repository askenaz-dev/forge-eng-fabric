from __future__ import annotations

import hashlib
import uuid
from dataclasses import dataclass

from rag_ingest.embeddings import EmbeddingsClient
from rag_ingest.milvus import VectorSink, collection_name_for
from rag_ingest.models import ChunkRecord, IngestRequest, IngestResponse, SourceDocument
from rag_ingest.signing import verify_provenance


@dataclass(frozen=True)
class IngestPipeline:
    embeddings: EmbeddingsClient
    sink: VectorSink
    provenance_secret: str
    chunk_size: int = 1600
    chunk_overlap: int = 160

    async def ingest(self, request: IngestRequest) -> IngestResponse:
        accepted: list[SourceDocument] = []
        rejected: list[dict[str, str]] = []
        for document in request.documents:
            signed = verify_provenance(
                secret=self.provenance_secret,
                source_ref=document.source_ref,
                text=document.text,
                signature=document.provenance_signature,
            )
            if request.require_signed_sources and not signed:
                rejected.append({"source_ref": document.source_ref, "reason": "invalid provenance signature"})
                continue
            accepted.append(document)

        chunks: list[ChunkRecord] = []
        for document in accepted:
            texts = chunk_text(document.text, chunk_size=self.chunk_size, chunk_overlap=self.chunk_overlap)
            embeddings = await self.embeddings.embed(texts)
            signed = verify_provenance(
                secret=self.provenance_secret,
                source_ref=document.source_ref,
                text=document.text,
                signature=document.provenance_signature,
            )
            for index, (text, embedding) in enumerate(zip(texts, embeddings, strict=True)):
                chunks.append(_chunk_record(document, text, embedding, index, signed))
        if chunks:
            await self.sink.write(chunks)
        return IngestResponse(
            accepted_documents=len(accepted),
            rejected_documents=len(rejected),
            chunks_written=len(chunks),
            rejected=rejected,
        )


def chunk_text(text: str, *, chunk_size: int, chunk_overlap: int) -> list[str]:
    normalized = "\n".join(line.rstrip() for line in text.splitlines()).strip()
    if not normalized:
        return []
    chunks: list[str] = []
    start = 0
    while start < len(normalized):
        end = min(start + chunk_size, len(normalized))
        chunks.append(normalized[start:end])
        if end == len(normalized):
            break
        start = max(end - chunk_overlap, start + 1)
    return chunks


def _chunk_record(
    document: SourceDocument,
    text: str,
    embedding: list[float],
    index: int,
    provenance_signed: bool,
) -> ChunkRecord:
    raw_id = f"{document.source_ref}:{index}:{hashlib.sha256(text.encode('utf-8')).hexdigest()}"
    chunk_id = str(uuid.uuid5(uuid.NAMESPACE_URL, raw_id))
    return ChunkRecord(
        chunk_id=chunk_id,
        workspace_id=document.workspace_id,
        tenant_id=document.tenant_id,
        collection_name=collection_name_for(workspace_id=document.workspace_id, visibility=document.visibility),
        text=text,
        embedding=embedding,
        visibility=document.visibility,
        data_classification=document.data_classification,
        source_kind=document.source_kind,
        source_ref=document.source_ref,
        provenance_signed=provenance_signed,
        metadata=document.metadata,
    )
