from __future__ import annotations

import uuid
from datetime import datetime, timedelta
from typing import Any, Literal

from pydantic import BaseModel, Field

ApprovalStatus = Literal["pending", "approved", "rejected", "expired"]


class ApprovalCreate(BaseModel):
    principal: str
    action: str
    workspace_id: uuid.UUID
    openspec_id: str | None = None
    target: dict[str, Any] = Field(default_factory=dict)
    rationale: str
    required_approvers: list[str]
    criticality: str = "low"
    correlation_id: str
    expiration_minutes: int = Field(default=240, ge=1, le=10080)


class ApprovalRequest(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    principal: str
    action: str
    workspace_id: uuid.UUID
    openspec_id: str | None = None
    target: dict[str, Any] = Field(default_factory=dict)
    rationale: str
    required_approvers: list[str]
    criticality: str
    correlation_id: str
    status: ApprovalStatus = "pending"
    requested_at: datetime = Field(default_factory=datetime.utcnow)
    expires_at: datetime
    decided_by: str | None = None
    decided_at: datetime | None = None
    decision_comment: str | None = None

    @classmethod
    def from_create(cls, request: ApprovalCreate) -> ApprovalRequest:
        return cls(
            principal=request.principal,
            action=request.action,
            workspace_id=request.workspace_id,
            openspec_id=request.openspec_id,
            target=request.target,
            rationale=request.rationale,
            required_approvers=request.required_approvers,
            criticality=request.criticality,
            correlation_id=request.correlation_id,
            expires_at=datetime.utcnow() + timedelta(minutes=request.expiration_minutes),
        )


class ApprovalDecision(BaseModel):
    actor: str
    decision: Literal["approved", "rejected"]
    comment: str | None = None


class ApprovalListResponse(BaseModel):
    approvals: list[ApprovalRequest]
