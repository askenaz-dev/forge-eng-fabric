"""Pydantic models for Alfred agent-mode sessions, steps and plan revisions."""

from __future__ import annotations

import uuid
from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field

SessionStatus = Literal[
    "planning",
    "running",
    "paused_for_approval",
    "paused_for_budget",
    "completed",
    "aborted",
    "failed",
]

StepKind = Literal["plan", "tool", "workflow", "agent", "approval", "final"]

StepStatus = Literal[
    "pending",
    "running",
    "paused_for_approval",
    "paused_for_budget",
    "succeeded",
    "failed",
    "skipped",
    "cancelled",
]


class AgentModeStep(BaseModel):
    """A single step in an agent-mode plan. Persists to `alfred_agent_step`."""

    model_config = ConfigDict(populate_by_name=True)

    id: uuid.UUID = Field(default_factory=uuid.uuid4)
    session_id: uuid.UUID
    idx: int
    kind: StepKind
    tool_id: str | None = None
    workflow_id: str | None = None
    agent_id: str | None = None
    criticality: Literal["low", "medium", "high", "critical"] = "low"
    decision_id: uuid.UUID | None = None
    status: StepStatus = "pending"
    started_at: datetime | None = None
    completed_at: datetime | None = None
    outcome: dict[str, Any] | None = None
    # Free-form params for the dispatcher (tool params, workflow input, etc.)
    params: dict[str, Any] = Field(default_factory=dict)
    summary: str = ""


TriggerSource = Literal["human", "symptom", "playbook", "replan"]


class AgentModeSession(BaseModel):
    """One agent-mode supervisory session. Persists to `alfred_agent_session`."""

    model_config = ConfigDict(populate_by_name=True)

    id: uuid.UUID = Field(default_factory=uuid.uuid4)
    workspace_id: uuid.UUID
    openspec_id: str | None = None
    correlation_id: str
    originator_principal: str
    model_id: str
    plan_revision: int = 1
    plan_json: dict[str, Any] = Field(default_factory=dict)
    frozen_autonomy_policy: dict[str, Any] = Field(default_factory=dict)
    status: SessionStatus = "planning"
    started_at: datetime = Field(default_factory=datetime.utcnow)
    paused_at: datetime | None = None
    resumed_at: datetime | None = None
    completed_at: datetime | None = None
    aborted_reason: str | None = None
    workflow_run_id: str | None = None

    # Non-human trigger fields (iter 2+)
    trigger_source: TriggerSource = "human"
    actor: str = ""
    actor_session: str | None = None
    symptom_id: str | None = None
    playbook_id: str | None = None
    parent_session_id: uuid.UUID | None = None


class PlanRevision(BaseModel):
    """A historical snapshot of a session's plan. Replanning increments revision."""

    revision: int
    plan_json: dict[str, Any]
    reason: str
    created_at: datetime = Field(default_factory=datetime.utcnow)
    inserted_step_idx: int | None = None


ALLOWED_START_STEPS = frozenset(
    {"discovery", "architect", "design", "test", "iac", "deploy"}
)

COMMITTED_LIFECYCLE_STATES = frozenset({"approved", "committed"})


class StartSessionRequest(BaseModel):
    workspace_id: uuid.UUID
    openspec_id: str | None = None
    intent: str | None = None
    correlation_id: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)
    # alfred-console-redesign 7.1: optional jump to a specific SDLC step.
    start_step: str | None = None

    # Non-human trigger fields (iter 2+).
    # trigger_source != "human" MUST have actor="system:alfred" and a non-null symptom_id.
    trigger_source: TriggerSource = "human"
    actor: str = ""
    actor_session: str | None = None
    symptom_id: str | None = None
    playbook_id: str | None = None
    parent_session_id: uuid.UUID | None = None
    autonomy_preset: str | None = None  # overrides workspace preset when actor=system:alfred


class FollowUpRequest(BaseModel):
    intent: str
    correlation_id: str | None = None
