from __future__ import annotations

import jsonschema
from fastapi import FastAPI, HTTPException, Query

from prompt_registry.models import (
    PromoteRequest,
    PromptTemplate,
    PromptTemplateCreate,
    RenderRequest,
    RenderResponse,
)
from prompt_registry.render import render_template
from prompt_registry.store import InMemoryPromptStore


def create_app(store: InMemoryPromptStore | None = None) -> FastAPI:
    store = store or InMemoryPromptStore()
    app = FastAPI(title="Prompt Registry", version="0.1.0")
    app.state.store = store

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/v1/templates")
    async def list_templates() -> dict[str, list[PromptTemplate]]:
        return {"templates": store.list()}

    @app.post("/v1/templates", response_model=PromptTemplate, status_code=201)
    async def create_template(request: PromptTemplateCreate) -> PromptTemplate:
        try:
            return store.create(request)
        except ValueError as exc:
            raise HTTPException(status_code=409, detail=str(exc)) from exc

    @app.get("/v1/templates/{template_id}", response_model=PromptTemplate)
    async def get_template(template_id: str, version: str | None = Query(default=None)) -> PromptTemplate:
        template = store.get(template_id, version)
        if not template:
            raise HTTPException(status_code=404, detail="template not found")
        return template

    @app.post("/v1/templates/{template_id}/render", response_model=RenderResponse)
    async def render(template_id: str, request: RenderRequest, version: str | None = Query(default=None)) -> RenderResponse:
        template = store.get(template_id, version)
        if not template:
            raise HTTPException(status_code=404, detail="template not found")
        try:
            rendered = render_template(template, request.variables)
        except jsonschema.ValidationError as exc:
            raise HTTPException(status_code=400, detail=f"invalid variables: {exc.message}") from exc
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc
        return RenderResponse(rendered=rendered, guardrails=template.guardrails)

    @app.post("/v1/templates/{template_id}/versions/{version}/promote", response_model=PromptTemplate)
    async def promote(template_id: str, version: str, request: PromoteRequest) -> PromptTemplate:
        try:
            template = store.promote(template_id, version, request)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc
        if not template:
            raise HTTPException(status_code=404, detail="template not found")
        return template

    return app

app = create_app()
