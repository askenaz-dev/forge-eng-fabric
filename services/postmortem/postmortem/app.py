"""FastAPI app for the postmortem generator."""

from __future__ import annotations

from fastapi import FastAPI

from .events import LogSink, Sink
from .generator import PostmortemGenerator
from .models import PostmortemRequest


def create_app(generator: PostmortemGenerator | None = None, sink: Sink | None = None) -> FastAPI:
    if generator is None:
        generator = PostmortemGenerator(sink=sink or LogSink())
    app = FastAPI(title="postmortem", version="0.1.0")
    app.state.generator = generator

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/postmortem/generate")
    def generate(req: PostmortemRequest):
        return app.state.generator.generate(req)

    @app.post("/v1/postmortem/publish")
    def publish(req: PostmortemRequest):
        gen: PostmortemGenerator = app.state.generator
        draft = gen.generate(req)
        evaluation = gen.evaluate(draft, req)
        if not evaluation["passed"]:
            return {"draft": draft, "evaluation": evaluation, "published": False}
        result = gen.publish(draft, req)
        return {"draft": draft, "evaluation": evaluation, "published": True, "result": result}

    return app


app = create_app()
