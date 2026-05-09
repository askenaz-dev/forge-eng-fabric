from __future__ import annotations

import uuid
from pathlib import Path

from fastapi import FastAPI, HTTPException, Query
from pydantic import BaseModel
from pydantic_settings import BaseSettings, SettingsConfigDict

from openspec_service.drafts import (
    DraftStore,
    can_commit,
    compute_completeness,
    to_create_request,
)
from openspec_service.events import EventPublisher, InMemoryEventPublisher
from openspec_service.models import (
    CompletenessReport,
    DecisionLogEntry,
    EvolutionLoopStats,
    IntentDraft,
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
    drafts_root: str = "openspec/drafts"


class LinkRequest(BaseModel):
    actor: str
    link: LinkedArtifact


class IntentStartRequest(BaseModel):
    workspace_id: uuid.UUID
    created_by: str
    title: str = ""
    business_intent: str = ""
    correlation_id: str | None = None


class IntentAnswerRequest(BaseModel):
    answer: str
    actor: str
    field_updates: dict = {}
    next_question: str | None = None


class IntentCommitRequest(BaseModel):
    actor: str


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
    drafts: DraftStore | None = None,
) -> FastAPI:
    settings = settings or Settings()
    store = store or FilesystemOpenSpecStore(Path(settings.openspec_root))
    drafts = drafts or DraftStore(root=Path(settings.drafts_root))
    publisher = publisher or InMemoryEventPublisher()
    app = FastAPI(title="OpenSpec Service", version="0.1.0")
    app.state.store = store
    app.state.publisher = publisher
    app.state.drafts = drafts

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

    @app.get("/v1/openspecs/{openspec_id}/completeness", response_model=CompletenessReport)
    async def openspec_completeness(openspec_id: str) -> CompletenessReport:
        # Drafts are looked up by draft_id; committed OpenSpecs by openspec_id.
        # The wizard polls this on the active draft only, but committed records
        # should still respond so the UI can show a fully-green completeness view.
        draft = drafts.get(openspec_id)
        if draft:
            return compute_completeness(draft)
        document = store.get(openspec_id)
        if not document:
            raise HTTPException(status_code=404, detail="openspec not found")
        synthetic = IntentDraft(
            draft_id=document.openspec_id,
            workspace_id=document.workspace_id,
            openspec_id=document.openspec_id,
            status="committed",
            title=document.title,
            business_intent=document.business_intent,
            problem_statement=document.problem_statement,
            stakeholders=document.stakeholders,
            success_metrics=document.success_metrics,
            requirements=document.requirements,
            constraints=document.constraints,
            autonomy_policy=document.autonomy_policy,
            created_by=document.audit.created_by,
        )
        return compute_completeness(synthetic)

    @app.post("/v1/intent/start", response_model=IntentDraft, status_code=201)
    async def intent_start(request: IntentStartRequest) -> IntentDraft:
        draft = drafts.create(
            workspace_id=request.workspace_id,
            created_by=request.created_by,
            title=request.title,
            business_intent=request.business_intent,
            correlation_id=request.correlation_id,
        )
        publisher.publish(
            "intent.dialogue.started.v1",
            draft.draft_id,
            {
                "draft_id": draft.draft_id,
                "workspace_id": str(draft.workspace_id),
                "actor": request.created_by,
                "correlation_id": request.correlation_id,
            },
        )
        return draft

    @app.get("/v1/intent/{draft_id}", response_model=IntentDraft)
    async def intent_get(draft_id: str) -> IntentDraft:
        draft = drafts.get(draft_id)
        if not draft:
            raise HTTPException(status_code=404, detail="draft not found")
        return draft

    @app.post("/v1/intent/{draft_id}/answer", response_model=IntentDraft)
    async def intent_answer(draft_id: str, request: IntentAnswerRequest) -> IntentDraft:
        draft = drafts.get(draft_id)
        if not draft:
            raise HTTPException(status_code=404, detail="draft not found")
        if draft.status != "drafting":
            raise HTTPException(status_code=409, detail=f"draft is {draft.status}; cannot accept answers")
        updated = drafts.append_turn(
            draft_id,
            question=request.next_question,
            answer=request.answer,
            field_updates=request.field_updates,
            actor=request.actor,
        )
        if updated is None:
            raise HTTPException(status_code=404, detail="draft not found")
        publisher.publish(
            "intent.dialogue.turn.v1",
            updated.draft_id,
            {
                "draft_id": updated.draft_id,
                "turn": updated.turn_count,
                "actor": request.actor,
                "field_updates": request.field_updates,
                "correlation_id": updated.correlation_id,
            },
        )
        return updated

    @app.post("/v1/intent/{draft_id}/commit", response_model=OpenSpecDocument)
    async def intent_commit(draft_id: str, request: IntentCommitRequest) -> OpenSpecDocument:
        draft = drafts.get(draft_id)
        if not draft:
            raise HTTPException(status_code=404, detail="draft not found")
        ok, reason = can_commit(draft)
        if not ok:
            raise HTTPException(status_code=400, detail=f"draft not commit-ready: {reason}")
        try:
            document = store.create(to_create_request(draft))
        except ValueError as exc:
            raise HTTPException(status_code=409, detail=str(exc)) from exc
        # Atomic flip: persist the canonical openspec_id back onto the draft and
        # mark it committed so subsequent GETs are aligned.
        draft.openspec_id = document.openspec_id
        draft.status = "committed"
        drafts.update(draft)
        publisher.publish(
            "intent.committed.v1",
            document.openspec_id,
            {
                "openspec_id": document.openspec_id,
                "draft_id": draft.draft_id,
                "workspace_id": str(document.workspace_id),
                "actor": request.actor,
                "correlation_id": draft.correlation_id,
            },
        )
        return document

    @app.post("/v1/intent/{draft_id}/abandon", response_model=IntentDraft)
    async def intent_abandon(draft_id: str) -> IntentDraft:
        draft = drafts.get(draft_id)
        if not draft:
            raise HTTPException(status_code=404, detail="draft not found")
        draft.status = "abandoned"
        drafts.update(draft)
        publisher.publish(
            "intent.draft.abandoned.v1",
            draft.draft_id,
            {"draft_id": draft.draft_id, "workspace_id": str(draft.workspace_id), "manual": True},
        )
        return draft

    @app.post("/v1/intent/expire-inactive")
    async def intent_expire_inactive() -> dict:
        flipped = drafts.expire_inactive()
        for draft in flipped:
            publisher.publish(
                "intent.draft.abandoned.v1",
                draft.draft_id,
                {"draft_id": draft.draft_id, "workspace_id": str(draft.workspace_id), "manual": False},
            )
        return {"abandoned_count": len(flipped), "abandoned_ids": [d.draft_id for d in flipped]}

    return app


app = create_app()
