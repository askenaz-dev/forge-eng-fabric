from __future__ import annotations

from pathlib import Path

from fastapi import FastAPI, HTTPException, Query
from pydantic_settings import BaseSettings, SettingsConfigDict

from approvals.events import EventPublisher, InMemoryEventPublisher
from approvals.models import ApprovalCreate, ApprovalDecision, ApprovalListResponse, ApprovalRequest
from approvals.store import FileApprovalStore, InMemoryApprovalStore


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="", extra="ignore")

    approvals_store_path: str = "data/approvals.json"


def create_app(
    store: InMemoryApprovalStore | None = None,
    publisher: EventPublisher | None = None,
    settings: Settings | None = None,
) -> FastAPI:
    settings = settings or Settings()
    store = store or FileApprovalStore(Path(settings.approvals_store_path))
    publisher = publisher or InMemoryEventPublisher()
    app = FastAPI(title="Approvals", version="0.1.0")
    app.state.store = store
    app.state.publisher = publisher

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/approvals", response_model=ApprovalRequest, status_code=201)
    async def create_approval(request: ApprovalCreate) -> ApprovalRequest:
        approval = store.create(request)
        publisher.publish(
            "approval.requested.v1",
            approval.id,
            approval.model_dump(mode="json"),
        )
        # Notification hooks are event-backed in Phase 1; email/Slack workers consume this event.
        publisher.publish(
            "approval.notification.queued.v1",
            approval.id,
            {"required_approvers": approval.required_approvers, "approval_id": approval.id},
        )
        return approval

    @app.get("/v1/approvals", response_model=ApprovalListResponse)
    async def list_approvals(
        approver: str | None = Query(default=None),
        status: str | None = Query(default=None),
        workspace_id: str | None = Query(default=None),
    ) -> ApprovalListResponse:
        return ApprovalListResponse(approvals=store.list(approver=approver, status=status, workspace_id=workspace_id))

    @app.post("/v1/approvals/{approval_id}/decisions", response_model=ApprovalRequest)
    async def decide(approval_id: str, decision: ApprovalDecision) -> ApprovalRequest:
        approval = store.decide(approval_id, decision)
        if not approval:
            raise HTTPException(status_code=404, detail="approval not found")
        publisher.publish(
            "approval.decided.v1",
            approval.id,
            approval.model_dump(mode="json"),
        )
        return approval

    @app.post("/v1/approvals/expire")
    async def expire_due() -> dict[str, int]:
        expired = store.expire_due()
        for approval in expired:
            publisher.publish("approval.expired.v1", approval.id, approval.model_dump(mode="json"))
        return {"expired": len(expired)}

    return app


app = create_app()
