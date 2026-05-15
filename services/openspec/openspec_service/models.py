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


class OpenSpecArtifactRef(BaseModel):
    change_id: str
    root: str
    files: list[str] = Field(default_factory=list)


SourceMarker = Literal["human", "autonomous-loop"]
ReviewStatus = Literal["pending", "approved", "rejected"]
LifecycleStatus = Literal["drafting", "validating", "committed", "abandoned"]


class CompletenessSection(BaseModel):
    """Per-section/field status surfaced to the wizard so it knows what to ask next."""

    name: str
    status: Literal["complete", "partial", "empty"]
    fields: dict[str, Literal["complete", "partial", "empty"]] = Field(default_factory=dict)


class CompletenessReport(BaseModel):
    openspec_id: str
    overall: Literal["complete", "partial", "empty"]
    sections: list[CompletenessSection]


class IntentDraft(BaseModel):
    """Progressive OpenSpec draft used by the Alfred wizard.

    Drafts share the `openspec_id` namespace with committed OpenSpecs so the
    wizard's commit step is atomic — when validation passes, the draft becomes a
    normal first-class OpenSpec.
    """

    draft_id: str
    workspace_id: uuid.UUID
    # Phase 5 (app-first-class-entity): every spec belongs to exactly one App.
    # Drafts carry an optional app_id while the wizard's first step
    # (`app_scope`) collects the scope; commit refuses if app_id is unset or
    # points at the `_unassigned` bucket.
    app_id: uuid.UUID | None = None
    openspec_id: str | None = None
    status: LifecycleStatus = "drafting"
    title: str = ""
    business_intent: str = ""
    problem_statement: str = ""
    stakeholders: list[str] = Field(default_factory=list)
    success_metrics: list[str] = Field(default_factory=list)
    requirements: Requirements = Field(default_factory=Requirements)
    constraints: list[str] = Field(default_factory=list)
    autonomy_policy: AutonomyPolicy = Field(default_factory=AutonomyPolicy)
    created_by: str
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)
    last_active_at: datetime = Field(default_factory=datetime.utcnow)
    turn_count: int = 0
    dialogue_history: list[dict[str, Any]] = Field(default_factory=list)
    correlation_id: str | None = None


class IntentAnswer(BaseModel):
    """A single user answer being applied to a draft."""

    draft_id: str
    answer: str
    field_updates: dict[str, Any] = Field(default_factory=dict)
    actor: str


class OpenSpecDocument(BaseModel):
    openspec_id: str
    workspace_id: uuid.UUID
    # Phase 5 (app-first-class-entity): every spec belongs to exactly one App.
    # The field is optional in the model so existing records remain valid;
    # the persistence layer rejects writes without app_id once
    # `forge.app_entity.enabled` is on for the spec's workspace.
    app_id: uuid.UUID | None = None
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
    # Draft lifecycle (platform-gaps-closure, openspec-backbone). Existing
    # records default to `committed` so historical OpenSpecs remain valid for
    # production-relevant operations without explicit migration of every row.
    lifecycle_status: LifecycleStatus = "committed"
    openspec_artifacts: OpenSpecArtifactRef | None = None

    @model_validator(mode="after")
    def validate_minimum_model(self) -> OpenSpecDocument:
        if not self.business_intent.strip():
            raise ValueError("business_intent is required")
        if not self.requirements.functional:
            raise ValueError("requirements.functional must contain at least one item")
        return self


class OpenSpecCreate(BaseModel):
    workspace_id: uuid.UUID
    # Optional during the M1-M4 migration window; the persistence layer
    # promotes this to required (`missing_app_scope`) for workspaces with
    # `forge.app_entity.enabled=true`.
    app_id: uuid.UUID | None = None
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


class OpenSpecReparentRequest(BaseModel):
    """Body for `POST /v1/specs/{id}:reparent` (app-first-class-entity 5.4).

    The platform must hold `app#editor` on both the source and target App for
    the call to succeed. The handler emits `spec.reparented.v1` with the
    correlation id captured here.
    """

    target_app_id: uuid.UUID
    reason: str
    actor: str
    correlation_id: str | None = None


class EvolutionLoopStats(BaseModel):
    """Counts for the evolution loop dashboard (task 11.3)."""

    total: int = 0
    pending: int = 0
    approved: int = 0
    rejected: int = 0
    acceptance_ratio: float = 0.0


class OpenSpecListResponse(BaseModel):
    openspecs: list[OpenSpecDocument]
