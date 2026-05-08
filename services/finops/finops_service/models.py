from __future__ import annotations

import uuid
from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator


REQUIRED_BILLING_TAGS = {"workspace", "env", "asset", "initiative_openspec"}
DEFAULT_THRESHOLDS = [50, 80, 100]


class BillingExportRecord(BaseModel):
    cost_usd: float
    currency: str = "USD"
    service: str
    usage_start: datetime = Field(default_factory=datetime.utcnow)
    tags: dict[str, str]

    @model_validator(mode="after")
    def require_tags(self) -> BillingExportRecord:
        missing = REQUIRED_BILLING_TAGS.difference(self.tags)
        if missing:
            raise ValueError(f"missing billing tags: {', '.join(sorted(missing))}")
        return self


class LLMCostRecord(BaseModel):
    source: Literal["langfuse", "litellm"]
    initiative_openspec: str
    workspace: str
    model: str
    cost_usd: float
    tokens: int = 0
    observed_at: datetime = Field(default_factory=datetime.utcnow)


class CostRecord(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    workspace_id: str
    initiative_openspec: str
    env: str = "unknown"
    asset: str = "unknown"
    category: str
    source: str
    cost_usd: float
    metadata: dict[str, Any] = Field(default_factory=dict)
    observed_at: datetime = Field(default_factory=datetime.utcnow)


class Budget(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    workspace_id: str
    initiative_openspec: str
    monthly_limit_usd: float
    thresholds: list[int] = Field(default_factory=lambda: list(DEFAULT_THRESHOLDS))
    consumed_usd: float = 0.0
    emitted_thresholds: set[int] = Field(default_factory=set)


class BudgetAlert(BaseModel):
    event_type: str = "finops.budget.threshold_reached.v1"
    workspace_id: str
    initiative_openspec: str
    threshold: int
    consumed_usd: float
    monthly_limit_usd: float
    budget_id: str
