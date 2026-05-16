"""Pydantic models shared between routes, the loop, and persistence."""

from __future__ import annotations

import uuid
from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field

PolicyOutcome = Literal["allow", "requires_approval", "deny"]
ActionStatus = Literal["pending", "running", "succeeded", "failed", "approved", "rejected", "expired"]


class IntentRequest(BaseModel):
    workspace_id: uuid.UUID
    text: str
    metadata: dict[str, Any] = Field(default_factory=dict)
    openspec_id: str | None = None
    correlation_id: str | None = None
    # alfred-litellm-header-injection (G1): the tenant id is required so
    # LiteLLM calls can be attributed correctly. Optional on the wire only to
    # preserve existing fixtures; the HTTP layer derives it from the JWT
    # when omitted (and ultimately the LiteLLM client fails closed if empty).
    tenant_id: str | None = None


class MessageIn(BaseModel):
    role: Literal["user", "assistant", "system", "tool"] = "user"
    content: str
    tool_call_id: str | None = None


class Session(BaseModel):
    id: uuid.UUID
    workspace_id: uuid.UUID
    actor: str
    started_at: datetime
    last_activity_at: datetime
    status: Literal["open", "closed"] = "open"
    correlation_id: str
    metadata: dict[str, Any] = Field(default_factory=dict)
    # alfred-litellm-header-injection (G1): persisted so RequestContext
    # for any subsequent LLM call on this session can be reconstructed
    # without re-deriving from JWT.
    tenant_id: str = ""


class DecisionRecord(BaseModel):
    """One row in Alfred's decision log — emitted for each relevant action."""

    model_config = ConfigDict(populate_by_name=True)

    id: uuid.UUID = Field(default_factory=uuid.uuid4)
    session_id: uuid.UUID
    workspace_id: uuid.UUID
    actor: str
    correlation_id: str
    intent: str
    retrieved_refs: list[dict[str, Any]] = Field(default_factory=list)
    policy_evaluated: dict[str, Any] | None = None
    tool_kind: Literal["mcp", "skill", "prompt", "llm", "delegation"] | None = None
    tool_id: str | None = None
    params_redacted: dict[str, Any] = Field(default_factory=dict)
    outcome: ActionStatus = "pending"
    outcome_detail: dict[str, Any] | None = None
    occurred_at: datetime = Field(default_factory=lambda: datetime.utcnow())


class IntentResponse(BaseModel):
    session_id: uuid.UUID
    correlation_id: str
    decisions: list[DecisionRecord]
    final_message: str
    requires_approval: bool = False
    approval_request_id: str | None = None
