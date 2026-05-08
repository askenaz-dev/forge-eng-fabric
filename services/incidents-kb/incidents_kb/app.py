"""FastAPI app for the incidents KB."""

from __future__ import annotations

from fastapi import FastAPI

from .events import LogSink, Sink
from .models import IndexRequest, SimilarRequest
from .service import IncidentsKB


def create_app(kb: IncidentsKB | None = None, sink: Sink | None = None) -> FastAPI:
    if kb is None:
        kb = IncidentsKB(sink=sink or LogSink())
    app = FastAPI(title="incidents-kb", version="0.1.0")
    app.state.kb = kb

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/kb/incidents/index")
    def index(req: IndexRequest):
        return app.state.kb.index(req)

    @app.post("/v1/kb/incidents/similar")
    def similar(req: SimilarRequest):
        return {"results": app.state.kb.similar(req)}

    @app.post("/v1/kb/incidents/recurrent/{tenant_id}")
    def recurrent(tenant_id: str):
        return {"clusters": app.state.kb.detect_recurrent(tenant_id)}

    return app


app = create_app()
