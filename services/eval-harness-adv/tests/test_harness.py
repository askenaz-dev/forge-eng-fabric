"""Tests for the advanced eval harness."""

from __future__ import annotations

import pytest

from eval_harness_adv.events import MemorySink
from eval_harness_adv.models import (
    DatasetItem,
    StartABRequest,
    StartRegressionRequest,
)
from eval_harness_adv.service import EvalHarness
from eval_harness_adv.store import Store


def _harness() -> tuple[EvalHarness, MemorySink]:
    sink = MemorySink()
    return EvalHarness(store=Store(), sink=sink), sink


def _seed_dataset(harness: EvalHarness, items: int = 4) -> None:
    harness.register_dataset(
        asset_id="ds-7",
        version="1.0.0",
        tenant_id="t1",
        workspace_id="w1",
        description=None,
        trust_level="internal",
        items=[DatasetItem(input={"i": i}, expected={"o": i}) for i in range(items)],
    )


def test_dataset_immutable_per_version() -> None:
    harness, _ = _harness()
    _seed_dataset(harness)
    with pytest.raises(ValueError, match="already_exists"):
        _seed_dataset(harness)


def test_regression_passes_without_baseline() -> None:
    harness, sink = _harness()
    _seed_dataset(harness, items=4)
    run = harness.start_regression(
        StartRegressionRequest(
            tenant_id="t1",
            workspace_id="w1",
            workflow_id="wf-1",
            workflow_version="1.0.0",
            dataset_id="ds-7",
            dataset_version="1.0.0",
        )
    )
    for _ in range(4):
        run = harness.record_outcome(run_id=run.id, success=True, cost_usd=0.1, latency_ms=100, business_metric_value=None)
    assert run.outcome.value == "passed"
    assert sink.by_type("workflow.eval.run_completed.v1")


def test_regression_blocks_when_below_baseline() -> None:
    harness, sink = _harness()
    _seed_dataset(harness, items=4)
    run = harness.start_regression(
        StartRegressionRequest(
            tenant_id="t1",
            workspace_id="w1",
            workflow_id="wf-1",
            workflow_version="1.1.0",
            dataset_id="ds-7",
            dataset_version="1.0.0",
            baseline_value=1.0,
            delta_threshold=0.10,
        )
    )
    # 2/4 = 0.5, well below baseline 1.0 - threshold 0.1
    for success in (True, False, False, False):
        run = harness.record_outcome(run_id=run.id, success=success, cost_usd=None, latency_ms=None, business_metric_value=None)
    assert run.outcome.value == "regression_blocked"
    assert sink.by_type("workflow.publish.regression_blocked.v1")


def test_ab_run_completes_and_reports_significance() -> None:
    harness, sink = _harness()
    run = harness.start_ab(
        StartABRequest(
            tenant_id="t1",
            workspace_id="w1",
            workflow_id="wf-1",
            version_a="1.0.0",
            version_b="1.1.0",
            target_executions=4,
        )
    )
    for variant, success in (("a", True), ("a", True), ("b", False), ("b", False)):
        run = harness.record_ab_outcome(
            ab_run_id=run.id,
            variant=variant,
            success=success,
            cost_usd=None,
            latency_ms=None,
            business_metric_value=None,
        )
    assert run.completed
    assert sink.by_type("workflow.eval.ab_completed.v1")


def test_publish_allowed_logic() -> None:
    harness, _ = _harness()
    _seed_dataset(harness, items=2)
    run = harness.start_regression(
        StartRegressionRequest(
            tenant_id="t1",
            workspace_id="w1",
            workflow_id="wf-1",
            workflow_version="1.0.0",
            dataset_id="ds-7",
            dataset_version="1.0.0",
        )
    )
    harness.record_outcome(run_id=run.id, success=True, cost_usd=None, latency_ms=None, business_metric_value=None)
    harness.record_outcome(run_id=run.id, success=True, cost_usd=None, latency_ms=None, business_metric_value=None)
    allowed, info = harness.is_publish_allowed(workflow_id="wf-1", workflow_version="1.0.0")
    assert allowed
    assert "run_id" in info
