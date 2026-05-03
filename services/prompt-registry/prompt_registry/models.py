from __future__ import annotations

import re
from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

LIFECYCLE = Literal["proposed", "in_review", "approved", "deprecated", "retired"]
TRUST = Literal["T0", "T1", "T2", "T3", "T4", "T5"]
SEMVER = re.compile(r"^\d+\.\d+\.\d+$")


class PromptTemplateCreate(BaseModel):
    id: str
    version: str
    owner_team: str
    template: str
    variables_schema: dict[str, Any]
    output_schema: dict[str, Any] | None = None
    examples: list[dict[str, Any]]
    recommended_model: str
    cost_class: str = "standard"
    eval_suite: str = "default-deterministic"
    guardrails: dict[str, Any] = Field(default_factory=dict)
    trust_level: TRUST = "T0"

    @model_validator(mode="after")
    def validate_publishable_metadata(self) -> PromptTemplateCreate:
        if not SEMVER.match(self.version):
            raise ValueError("version must be SemVer (x.y.z)")
        if not self.examples:
            raise ValueError("at least one example is required")
        return self


class PromptTemplate(PromptTemplateCreate):
    lifecycle_state: LIFECYCLE = "proposed"
    eval_scores: dict[str, float] = Field(default_factory=dict)
    change_history: list[dict[str, Any]] = Field(default_factory=list)
    created_at: datetime = Field(default_factory=datetime.utcnow)


class RenderRequest(BaseModel):
    variables: dict[str, Any]


class RenderResponse(BaseModel):
    rendered: str
    guardrails: dict[str, Any]


class PromoteRequest(BaseModel):
    lifecycle_state: LIFECYCLE
    eval_scores: dict[str, float]
    actor: str
