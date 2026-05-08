from __future__ import annotations

import uuid
from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator


class Requirements(BaseModel):
    functional: list[str] = Field(default_factory=list)
    non_functional: list[str] = Field(default_factory=list)


class AutonomyPolicy(BaseModel):
    default_mode: Literal["autonomous", "requires_approval", "restricted"] = "autonomous"
    approvals_required: list[str] = Field(default_factory=list)


class LinkedArtifact(BaseModel):
    kind: str
    ref: str
    namespace: str | None = None
    direction: Literal["from_openspec", "to_openspec", "bidirectional"] = "bidirectional"
    metadata: dict[str, Any] = Field(default_factory=dict)

    @model_validator(mode="after")
    def derive_namespace(self) -> LinkedArtifact:
        if self.namespace:
            return self
        if self.ref and ":" in self.ref:
            self.namespace = self.ref.split(":", 1)[0]
        elif self.kind:
            self.namespace = self.kind.removesuffix(":")
        return self


class DecisionLogEntry(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    type: str = "decision"
    actor: str
    decision: str = ""
    rationale: str = ""
    timestamp: datetime = Field(default_factory=datetime.utcnow)
    policy: dict[str, Any] | None = None
    correlation_id: str | None = None
    key: str | None = None
    url: str | None = None
    status: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class AuditInfo(BaseModel):
    created_by: str
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_by: str | None = None
    updated_at: datetime | None = None


SourceMarker = Literal["human", "autonomous-loop"]
ReviewStatus = Literal["pending", "approved", "rejected"]


class OpenSpecDocument(BaseModel):
    openspec_id: str
    workspace_id: uuid.UUID
    title: str
    business_intent: str
    problem_statement: str
    stakeholders: list[str] = Field(default_factory=list)
    success_metrics: list[str] = Field(default_factory=list)
    requirements: Requirements
    constraints: list[str] = Field(default_factory=list)
    autonomy_policy: AutonomyPolicy = Field(default_factory=AutonomyPolicy)
    linked_artifacts: list[LinkedArtifact] = Field(default_factory=list)
    decision_log: list[DecisionLogEntry] = Field(default_factory=list)
    audit: AuditInfo
    version: int = 1
    # Phase 6: marker that distinguishes proposals derived by the autonomous
    # evolution loop from those authored by humans. UI shows a distinct badge.
    source: SourceMarker = "human"
    # Autonomous-loop docs start in `pending`. They cannot be treated as
    # accepted by downstream services until a human reviewer approves.
    review_status: ReviewStatus = "approved"
    reviewed_by: str | None = None
    review_comment: str | None = None

    @model_validator(mode="after")
    def validate_minimum_model(self) -> OpenSpecDocument:
        if not self.business_intent.strip():
            raise ValueError("business_intent is required")
        if not self.requirements.functional:
            raise ValueError("requirements.functional must contain at least one item")
        return self


class OpenSpecCreate(BaseModel):
    workspace_id: uuid.UUID
    title: str
    business_intent: str
    problem_statement: str
    stakeholders: list[str] = Field(default_factory=list)
    success_metrics: list[str] = Field(default_factory=list)
    requirements: Requirements
    constraints: list[str] = Field(default_factory=list)
    autonomy_policy: AutonomyPolicy = Field(default_factory=AutonomyPolicy)
    linked_artifacts: list[LinkedArtifact] = Field(default_factory=list)
    created_by: str
    openspec_id: str | None = None
    source: SourceMarker = "human"

    @model_validator(mode="after")
    def validate_minimum_model(self) -> OpenSpecCreate:
        if not self.business_intent.strip():
            raise ValueError("business_intent is required")
        if not self.requirements.functional:
            raise ValueError("requirements.functional must contain at least one item")
        return self


class OpenSpecPatch(BaseModel):
    title: str | None = None
    business_intent: str | None = None
    problem_statement: str | None = None
    stakeholders: list[str] | None = None
    success_metrics: list[str] | None = None
    requirements: Requirements | None = None
    constraints: list[str] | None = None
    autonomy_policy: AutonomyPolicy | None = None
    updated_by: str


class OpenSpecReviewRequest(BaseModel):
    """Body for the autonomous-loop review endpoint."""

    approved: bool
    reviewer: str
    comment: str | None = None


class EvolutionLoopStats(BaseModel):
    """Counts for the evolution loop dashboard (task 11.3)."""

    total: int = 0
    pending: int = 0
    approved: int = 0
    rejected: int = 0
    acceptance_ratio: float = 0.0


class OpenSpecListResponse(BaseModel):
    openspecs: list[OpenSpecDocument]
