from __future__ import annotations

from typing import Any

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from reference_skills.skills import create_user_stories, generate_test_cases, scaffold_service


class InvokeRequest(BaseModel):
    tool_id: str
    params: dict[str, Any] = Field(default_factory=dict)


class InvokeResponse(BaseModel):
    tool_id: str
    ok: bool = True
    result: dict[str, Any]


def create_app() -> FastAPI:
    app = FastAPI(title="Forge Reference Skill Runner", version="0.1.0")

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/invoke", response_model=InvokeResponse)
    async def invoke(request: InvokeRequest) -> InvokeResponse:
        if request.tool_id == "skill:create-user-stories":
            result = create_user_stories(_openspec_param(request.params))
        elif request.tool_id == "skill:generate-test-cases":
            result = generate_test_cases(_openspec_param(request.params))
        elif request.tool_id == "skill:scaffold-service":
            result = scaffold_service(
                name=str(request.params.get("name") or "forge-service"),
                language=str(request.params.get("language") or "python"),
            )
        else:
            raise HTTPException(status_code=404, detail="unknown skill")
        return InvokeResponse(tool_id=request.tool_id, result=result)

    return app


def _openspec_param(params: dict[str, Any]) -> dict[str, Any]:
    openspec = params.get("openspec") or params
    if not isinstance(openspec, dict):
        raise HTTPException(status_code=400, detail="openspec must be an object")
    return openspec


app = create_app()
