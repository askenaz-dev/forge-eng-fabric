"""FastAPI app that exposes the diagnosis pipeline."""

from __future__ import annotations

from fastapi import FastAPI, HTTPException

from .events import LogSink, Sink
from .models import DiagnosisRequest
from .pipeline import DiagnosisPipeline


def create_app(pipeline: DiagnosisPipeline | None = None, sink: Sink | None = None) -> FastAPI:
    if pipeline is None:
        pipeline = DiagnosisPipeline(sink=sink or LogSink())
    app = FastAPI(title="diagnosis", version="0.1.0")
    app.state.pipeline = pipeline

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/diagnose")
    def diagnose(req: DiagnosisRequest):
        p: DiagnosisPipeline = app.state.pipeline
        try:
            report = p.run(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc
        return report

    return app


app = create_app()
