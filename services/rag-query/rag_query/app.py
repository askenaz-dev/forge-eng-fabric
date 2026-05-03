from __future__ import annotations

from fastapi import FastAPI, HTTPException
from pydantic_settings import BaseSettings, SettingsConfigDict

from rag_query.authz import can_view_workspace
from rag_query.models import IndexedChunk, QueryRequest, QueryResponse
from rag_query.store import InMemoryQueryStore


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="", extra="ignore")

    openfga_url: str = "http://localhost:8088"
    openfga_store: str = ""
    openfga_model: str = ""


def create_app(settings: Settings | None = None, store: InMemoryQueryStore | None = None) -> FastAPI:
    settings = settings or Settings()
    store = store or InMemoryQueryStore()
    app = FastAPI(title="RAG Query", version="0.1.0")
    app.state.store = store

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/index")
    async def index(chunks: list[IndexedChunk]) -> dict[str, int]:
        store.upsert(chunks)
        return {"chunks": len(chunks)}

    @app.post("/v1/query", response_model=QueryResponse)
    async def query(request: QueryRequest) -> QueryResponse:
        allowed = await can_view_workspace(
            openfga_url=settings.openfga_url,
            store_id=settings.openfga_store,
            model_id=settings.openfga_model,
            principal=request.principal,
            workspace_id=request.workspace_id,
        )
        if not allowed:
            raise HTTPException(status_code=403, detail="workspace can_view required")
        return QueryResponse(results=store.query(request))

    return app


app = create_app()
