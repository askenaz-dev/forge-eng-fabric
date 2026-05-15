"""FastAPI app that exposes SDLC design skill endpoints."""

from __future__ import annotations

from fastapi import FastAPI, HTTPException

from .events import EventSink, LogSink
from .models import (
    AccessibilityAuditRequest,
    AccessibilityAuditResponse,
    ComponentStubsRequest,
    ComponentStubsResponse,
    UiBlueprintRequest,
    UiBlueprintResponse,
)
from .skills import accessibility_audit, generate_component_stubs, generate_ui_blueprint


def create_app(sink: EventSink | None = None) -> FastAPI:
    _sink = sink or LogSink()
    app = FastAPI(title="sdlc-design-skills", version="0.1.0")
    app.state.sink = _sink

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/skills/generate-ui-blueprint", response_model=UiBlueprintResponse)
    def skill_generate_ui_blueprint(req: UiBlueprintRequest) -> UiBlueprintResponse:
        try:
            return generate_ui_blueprint(req, app.state.sink)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/generate-component-stubs", response_model=ComponentStubsResponse)
    def skill_generate_component_stubs(req: ComponentStubsRequest) -> ComponentStubsResponse:
        try:
            return generate_component_stubs(req, app.state.sink)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/accessibility-audit", response_model=AccessibilityAuditResponse)
    def skill_accessibility_audit(req: AccessibilityAuditRequest) -> AccessibilityAuditResponse:
        try:
            return accessibility_audit(req, app.state.sink)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    return app


app = create_app()
