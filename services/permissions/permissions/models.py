from __future__ import annotations

import uuid
from datetime import datetime, timedelta
from typing import Literal

from pydantic import BaseModel, Field

Criticality = Literal["low", "medium", "high", "critical"]
GrantStatus = Literal["active", "revoked", "expired"]


class GrantCreate(BaseModel):
    subject: str = "alfred"
    scope_kind: Literal["workspace", "repo", "environment", "cloud_project"]
    scope_id: str
    action_class: str
    max_criticality: Criticality = "medium"
    expiration_days: int = Field(default=30, ge=1, le=365)
    justification: str
    requester: str
    approver: str


class AuditEntry(BaseModel):
    actor: str
    action: str
    timestamp: datetime = Field(default_factory=datetime.utcnow)
    rationale: str | None = None


class DelegatedPermission(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    subject: str
    scope_kind: str
    scope_id: str
    action_class: str
    max_criticality: Criticality
    expires_at: datetime
    justification: str
    requester: str
    approver: str
    status: GrantStatus = "active"
    openfga_tuple: dict[str, str]
    audit_history: list[AuditEntry]

    @classmethod
    def from_create(cls, request: GrantCreate) -> DelegatedPermission:
        return cls(
            subject=request.subject,
            scope_kind=request.scope_kind,
            scope_id=request.scope_id,
            action_class=request.action_class,
            max_criticality=request.max_criticality,
            expires_at=datetime.utcnow() + timedelta(days=request.expiration_days),
            justification=request.justification,
            requester=request.requester,
            approver=request.approver,
            openfga_tuple={
                "user": f"agent:{request.subject}",
                "relation": request.action_class.replace(":", "_"),
                "object": f"{request.scope_kind}:{request.scope_id}",
            },
            audit_history=[AuditEntry(actor=request.approver, action="granted", rationale=request.justification)],
        )


class RevokeRequest(BaseModel):
    actor: str
    rationale: str


class CheckRequest(BaseModel):
    subject: str
    action_class: str
    scope_kind: str
    scope_id: str
    criticality: Criticality = "low"


class CheckResponse(BaseModel):
    allowed: bool
    reason: str
    grant_id: str | None = None
