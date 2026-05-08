"""Models for the FinOps advisor."""

from __future__ import annotations

from datetime import datetime, timezone
from enum import Enum
from typing import Any

from pydantic import BaseModel, Field


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


class RecommendationKind(str, Enum):
    DOWNSIZE_RESOURCE = "downsize_resource"
    IDLE_RESOURCE = "idle_resource"
    OVERSIZED_RESOURCE = "oversized_resource"
    EXPENSIVE_LLM_SKILL = "expensive_llm_skill"
    CACHEABLE_PROMPT = "cacheable_prompt"


class CostRecord(BaseModel):
    tenant_id: str
    workspace_id: str | None = None
    asset_id: str | None = None
    service: str | None = None
    skill_id: str | None = None
    resource_id: str | None = None
    kind: str  # cloud | llm
    spend_usd: float
    utilization: float | None = None  # 0..1 for cloud resources
    invocations: int | None = None
    cache_hit_rate: float | None = None
    timestamp: datetime = Field(default_factory=utcnow)


class Recommendation(BaseModel):
    id: str
    tenant_id: str
    workspace_id: str | None = None
    asset_id: str | None = None
    kind: RecommendationKind
    title: str
    detail: str
    expected_savings_usd_monthly: float
    affected_resources: list[str] = Field(default_factory=list)
    pr_url: str | None = None
    pr_status: str = "draft"  # draft | open | merged | rejected
    severity: str = "medium"
    metadata: dict[str, Any] = Field(default_factory=dict)
    created_at: datetime = Field(default_factory=utcnow)
    synthetic: bool = False


class RecommendationsRequest(BaseModel):
    tenant_id: str
    records: list[CostRecord]
    synthetic: bool = False
