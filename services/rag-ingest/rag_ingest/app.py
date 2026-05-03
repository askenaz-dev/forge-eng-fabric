from __future__ import annotations

import uuid
from pathlib import Path

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from pydantic_settings import BaseSettings, SettingsConfigDict

from rag_ingest.connectors import FilesystemOpenSpecConnector
from rag_ingest.embeddings import DeterministicEmbeddings, LiteLLMEmbeddings
from rag_ingest.milvus import InMemoryVectorSink, MilvusVectorSink, VectorSink
from rag_ingest.models import IngestRequest, IngestResponse
from rag_ingest.pipeline import IngestPipeline


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="", extra="ignore")

    litellm_url: str = "http://localhost:4000"
    litellm_key: str = "sk-forge-local"
    embedding_model: str = "text-embedding-004"
    milvus_uri: str = ""
    milvus_token: str | None = None
    provenance_secret: str = "forge-local-provenance"
    use_deterministic_embeddings: bool = False


class FilesystemIngestRequest(BaseModel):
    root_path: str
    workspace_id: uuid.UUID
    tenant_id: uuid.UUID | None = None
    require_signed_sources: bool = False


def create_app(settings: Settings | None = None, sink: VectorSink | None = None) -> FastAPI:
    settings = settings or Settings()
    embeddings = (
        DeterministicEmbeddings()
        if settings.use_deterministic_embeddings
        else LiteLLMEmbeddings(
            base_url=settings.litellm_url,
            api_key=settings.litellm_key,
            model=settings.embedding_model,
        )
    )
    vector_sink = sink or (MilvusVectorSink(settings.milvus_uri, settings.milvus_token) if settings.milvus_uri else InMemoryVectorSink())
    pipeline = IngestPipeline(
        embeddings=embeddings,
        sink=vector_sink,
        provenance_secret=settings.provenance_secret,
    )

    app = FastAPI(title="RAG Ingest", version="0.1.0")
    app.state.pipeline = pipeline
    app.state.sink = vector_sink

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/ingest", response_model=IngestResponse)
    async def ingest(request: IngestRequest) -> IngestResponse:
        return await pipeline.ingest(request)

    @app.post("/v1/ingest/filesystem", response_model=IngestResponse)
    async def ingest_filesystem(request: FilesystemIngestRequest) -> IngestResponse:
        root = Path(request.root_path).resolve()
        if not root.exists() or not root.is_dir():
            raise HTTPException(status_code=400, detail="root_path must be an existing directory")
        connector = FilesystemOpenSpecConnector(
            root=root,
            workspace_id=request.workspace_id,
            tenant_id=request.tenant_id,
        )
        docs = await connector.fetch()
        return await pipeline.ingest(
            IngestRequest(documents=docs, require_signed_sources=request.require_signed_sources)
        )

    @app.post("/v1/webhooks/{source_kind}", response_model=IngestResponse)
    async def ingest_webhook(source_kind: str, request: IngestRequest) -> IngestResponse:
        # The webhook path records source intent; document-level source_kind remains authoritative.
        if source_kind not in {"github", "confluence", "jira", "registry", "audit", "incident"}:
            raise HTTPException(status_code=404, detail="unsupported source kind")
        return await pipeline.ingest(request)

    @app.post("/v1/events/kafka", response_model=IngestResponse)
    async def ingest_event(request: IngestRequest) -> IngestResponse:
        return await pipeline.ingest(request)

    return app


app = create_app()
