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
    direction: Literal["from_openspec", "to_openspec", "bidirectional"] = "bidirectional"
    metadata: dict[str, Any] = Field(default_factory=dict)


class DecisionLogEntry(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    actor: str
    decision: str
    rationale: str
    timestamp: datetime = Field(default_factory=datetime.utcnow)
    policy: dict[str, Any] | None = None
    correlation_id: str | None = None


class AuditInfo(BaseModel):
    created_by: str
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_by: str | None = None
    updated_at: datetime | None = None


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


class OpenSpecListResponse(BaseModel):
    openspecs: list[OpenSpecDocument]
