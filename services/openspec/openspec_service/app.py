from __future__ import annotations

from pathlib import Path

from fastapi import FastAPI, HTTPException, Query
from pydantic import BaseModel
from pydantic_settings import BaseSettings, SettingsConfigDict

from openspec_service.events import EventPublisher, InMemoryEventPublisher
from openspec_service.models import (
    DecisionLogEntry,
    EvolutionLoopStats,
    LinkedArtifact,
    OpenSpecCreate,
    OpenSpecDocument,
    OpenSpecListResponse,
    OpenSpecPatch,
    OpenSpecReviewRequest,
)
from openspec_service.store import FilesystemOpenSpecStore


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="", extra="ignore")

    openspec_root: str = "openspec/records"


class LinkRequest(BaseModel):
    actor: str
    link: LinkedArtifact


class JiraHookRequest(BaseModel):
    openspec_id: str
    key: str
    url: str | None = None
    status: str | None = None
    actor: str = "jira"
    correlation_id: str | None = None


def create_app(
    settings: Settings | None = None,
    store: FilesystemOpenSpecStore | None = None,
    publisher: EventPublisher | None = None,
) -> FastAPI:
    settings = settings or Settings()
    store = store or FilesystemOpenSpecStore(Path(settings.openspec_root))
    publisher = publisher or InMemoryEventPublisher()
    app = FastAPI(title="OpenSpec Service", version="0.1.0")
    app.state.store = store
    app.state.publisher = publisher

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/v1/openspecs", response_model=OpenSpecListResponse)
    async def list_openspecs(workspace_id: str | None = Query(default=None)) -> OpenSpecListResponse:
        return OpenSpecListResponse(openspecs=store.list(workspace_id=workspace_id))

    @app.post("/v1/openspecs", response_model=OpenSpecDocument, status_code=201)
    async def create_openspec(request: OpenSpecCreate) -> OpenSpecDocument:
        try:
            document = store.create(request)
        except ValueError as exc:
            raise HTTPException(status_code=409, detail=str(exc)) from exc
        publisher.publish(
            "openspec.created.v1",
            document.openspec_id,
            {"openspec_id": document.openspec_id, "workspace_id": str(document.workspace_id)},
        )
        return document

    @app.get("/v1/openspecs/{openspec_id}", response_model=OpenSpecDocument)
    async def get_openspec(openspec_id: str) -> OpenSpecDocument:
        document = store.get(openspec_id)
        if not document:
            raise HTTPException(status_code=404, detail="openspec not found")
        return document

    @app.get("/v1/openspecs/{openspec_id}/versions")
    async def list_versions(openspec_id: str) -> dict[str, list[int]]:
        if not store.get(openspec_id):
            raise HTTPException(status_code=404, detail="openspec not found")
        return {"versions": store.list_versions(openspec_id)}

    @app.get("/v1/openspecs/{openspec_id}/versions/{version}", response_model=OpenSpecDocument)
    async def get_version(openspec_id: str, version: int) -> OpenSpecDocument:
        document = store.get_version(openspec_id, version)
        if not document:
            raise HTTPException(status_code=404, detail="openspec version not found")
        return document

    @app.patch("/v1/openspecs/{openspec_id}", response_model=OpenSpecDocument)
    async def patch_openspec(openspec_id: str, request: OpenSpecPatch) -> OpenSpecDocument:
        try:
            document = store.patch(openspec_id, request)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc
        if not document:
            raise HTTPException(status_code=404, detail="openspec not found")
        publisher.publish(
            "openspec.updated.v1",
            document.openspec_id,
            {"openspec_id": document.openspec_id, "version": document.version},
        )
        return document

    @app.post("/v1/openspecs/{openspec_id}/decisions", response_model=OpenSpecDocument)
    async def append_decision(openspec_id: str, request: DecisionLogEntry) -> OpenSpecDocument:
        document = store.append_decision(openspec_id, request)
        if not document:
            raise HTTPException(status_code=404, detail="openspec not found")
        publisher.publish(
            "openspec.updated.v1",
            document.openspec_id,
            {"openspec_id": document.openspec_id, "decision_id": request.id, "version": document.version},
        )
        return document

    @app.post("/v1/openspecs/{openspec_id}/links", response_model=OpenSpecDocument)
    async def append_link(openspec_id: str, request: LinkRequest) -> OpenSpecDocument:
        document = store.append_link(openspec_id, request.link, request.actor)
        if not document:
            raise HTTPException(status_code=404, detail="openspec not found")
        publisher.publish(
            "openspec.linked.v1",
            document.openspec_id,
            {"openspec_id": document.openspec_id, "link": request.link.model_dump(mode="json")},
        )
        return document

    @app.post("/v1/hooks/jira", response_model=OpenSpecDocument)
    async def jira_hook(request: JiraHookRequest) -> OpenSpecDocument:
        link = LinkedArtifact(
            kind="jira",
            namespace="jira",
            ref=f"jira:{request.key}",
            metadata={"url": request.url, "status": request.status},
        )
        document = store.append_link(request.openspec_id, link, request.actor)
        if not document:
            raise HTTPException(status_code=404, detail="openspec not found")
        decision = DecisionLogEntry(
            type="jira_link",
            actor=request.actor,
            decision=f"Jira issue {request.key} linked or updated",
            rationale="Jira webhook updated OpenSpec traceability",
            correlation_id=request.correlation_id,
            key=request.key,
            url=request.url,
            status=request.status,
        )
        document = store.append_decision(request.openspec_id, decision)
        publisher.publish(
            "openspec.updated.v1",
            document.openspec_id,
            {"openspec_id": document.openspec_id, "decision_id": decision.id, "link": link.model_dump(mode="json")},
        )
        return document

    @app.post("/v1/sync/filesystem")
    async def sync_filesystem() -> dict[str, int]:
        store.sync_from_filesystem()
        return {"openspecs": len(store.index.rows)}

    @app.post("/v1/openspecs/{openspec_id}/review", response_model=OpenSpecDocument)
    async def review_openspec(openspec_id: str, request: OpenSpecReviewRequest) -> OpenSpecDocument:
        try:
            document = store.review(
                openspec_id,
                approved=request.approved,
                reviewer=request.reviewer,
                comment=request.comment,
            )
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc
        if not document:
            raise HTTPException(status_code=404, detail="openspec not found")
        publisher.publish(
            "openspec.autonomous_loop.reviewed.v1",
            document.openspec_id,
            {
                "openspec_id": document.openspec_id,
                "approved": request.approved,
                "reviewer": request.reviewer,
                "version": document.version,
            },
        )
        return document

    @app.get("/v1/evolution/stats", response_model=EvolutionLoopStats)
    async def evolution_stats() -> EvolutionLoopStats:
        stats = store.evolution_stats()
        return EvolutionLoopStats(**stats)

    return app


app = create_app()
