"""Pydantic models for the advanced eval harness."""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from enum import Enum
from typing import Any

from pydantic import BaseModel, Field


def _now() -> datetime:
    return datetime.now(timezone.utc)


def _new_id() -> str:
    return str(uuid.uuid4())


class DatasetItem(BaseModel):
    """A single golden input/expected pair in an eval dataset."""

    id: str = Field(default_factory=_new_id)
    input: dict[str, Any]
    expected: dict[str, Any]
    weight: float = 1.0


class EvalDataset(BaseModel):
    """An immutable, versioned eval dataset registered as a Registry asset."""

    asset_id: str
    version: str
    tenant_id: str
    workspace_id: str
    description: str | None = None
    trust_level: str = "internal"
    items: list[DatasetItem] = Field(default_factory=list)
    created_at: datetime = Field(default_factory=_now)


class RunOutcome(str, Enum):
    PASSED = "passed"
    FAILED = "failed"
    REGRESSION_BLOCKED = "regression_blocked"


class EvalRun(BaseModel):
    """An execution of a dataset against a workflow/skill version."""

    id: str = Field(default_factory=_new_id)
    tenant_id: str
    workspace_id: str
    workflow_id: str
    workflow_version: str
    dataset_id: str
    dataset_version: str
    outcome: RunOutcome = RunOutcome.PASSED
    metric_key: str = "success_rate"
    metric_value: float = 0.0
    baseline_value: float | None = None
    delta_threshold: float = 0.03
    items: int = 0
    failures: int = 0
    cost_usd: float | None = None
    latency_p95_ms: float | None = None
    business_metric_value: float | None = None
    started_at: datetime = Field(default_factory=_now)
    completed_at: datetime | None = None


class ABRun(BaseModel):
    """An A/B comparison of two versions for an opt-in Workspace."""

    id: str = Field(default_factory=_new_id)
    tenant_id: str
    workspace_id: str
    workflow_id: str
    version_a: str
    version_b: str
    target_executions: int = 200
    counts: dict[str, int] = Field(default_factory=lambda: {"a": 0, "b": 0})
    metrics: dict[str, dict[str, float]] = Field(default_factory=dict)
    significant: bool = False
    completed: bool = False
    started_at: datetime = Field(default_factory=_now)
    completed_at: datetime | None = None


class RecordOutcomeRequest(BaseModel):
    """Used by external runners to feed individual evaluation outcomes."""

    workflow_id: str
    workflow_version: str
    dataset_id: str
    dataset_version: str
    success: bool
    cost_usd: float | None = None
    latency_ms: float | None = None
    business_metric_value: float | None = None
    expected: dict[str, Any] | None = None
    actual: dict[str, Any] | None = None


class CreateDatasetRequest(BaseModel):
    asset_id: str
    version: str
    tenant_id: str
    workspace_id: str
    description: str | None = None
    trust_level: str = "internal"
    items: list[DatasetItem] = Field(default_factory=list)


class StartRegressionRequest(BaseModel):
    tenant_id: str
    workspace_id: str
    workflow_id: str
    workflow_version: str
    dataset_id: str
    dataset_version: str
    metric_key: str = "success_rate"
    delta_threshold: float = 0.03
    baseline_value: float | None = None


class StartABRequest(BaseModel):
    tenant_id: str
    workspace_id: str
    workflow_id: str
    version_a: str
    version_b: str
    target_executions: int = 200


class RecordABOutcomeRequest(BaseModel):
    ab_run_id: str
    variant: str  # "a" | "b"
    success: bool
    cost_usd: float | None = None
    latency_ms: float | None = None
    business_metric_value: float | None = None
