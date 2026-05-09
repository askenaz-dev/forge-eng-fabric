"""Eval harness service logic."""

from __future__ import annotations

import math
from datetime import datetime, timezone
from typing import Any

from .events import CloudEvent, EventSink, MemorySink
from .models import (
    ABRun,
    DatasetItem,
    EvalDataset,
    EvalRun,
    RunOutcome,
    StartABRequest,
    StartRegressionRequest,
)
from .store import Store


class EvalHarness:
    """Coordinates regression runs, A/B comparisons and business metrics.

    The actual workflow execution is delegated to the runtime; this service
    is the source of truth for whether a publish is allowed (regression
    gate) and what an A/B comparison says.
    """

    def __init__(self, store: Store, sink: EventSink | None = None) -> None:
        self.store = store
        self.sink = sink or MemorySink()

    # Datasets ---------------------------------------------------------

    def register_dataset(
        self,
        *,
        asset_id: str,
        version: str,
        tenant_id: str,
        workspace_id: str,
        description: str | None,
        trust_level: str,
        items: list[DatasetItem],
    ) -> EvalDataset:
        dataset = EvalDataset(
            asset_id=asset_id,
            version=version,
            tenant_id=tenant_id,
            workspace_id=workspace_id,
            description=description,
            trust_level=trust_level,
            items=items,
        )
        self.store.upsert_dataset(dataset)
        self.sink.emit(
            CloudEvent.new(
                type_="asset.eval_dataset.registered.v1",
                tenant_id=tenant_id,
                workspace_id=workspace_id,
                subject=f"eval-dataset/{asset_id}@{version}",
                data={
                    "asset_id": asset_id,
                    "version": version,
                    "items": len(items),
                },
            )
        )
        return dataset

    # Regression runs --------------------------------------------------

    def start_regression(self, req: StartRegressionRequest) -> EvalRun:
        dataset = self.store.get_dataset(req.dataset_id, req.dataset_version)
        if dataset is None:
            raise ValueError("dataset_not_found")
        baseline = req.baseline_value
        if baseline is None:
            previous = self.store.latest_passing_run(req.workflow_id, req.dataset_id)
            baseline = previous.metric_value if previous else None
        run = EvalRun(
            tenant_id=req.tenant_id,
            workspace_id=req.workspace_id,
            workflow_id=req.workflow_id,
            workflow_version=req.workflow_version,
            dataset_id=req.dataset_id,
            dataset_version=req.dataset_version,
            metric_key=req.metric_key,
            delta_threshold=req.delta_threshold,
            baseline_value=baseline,
            items=len(dataset.items),
        )
        self.store.insert_run(run)
        self.sink.emit(
            CloudEvent.new(
                type_="workflow.eval.run_started.v1",
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                subject=f"eval-run/{run.id}",
                data={
                    "run_id": run.id,
                    "workflow_id": req.workflow_id,
                    "workflow_version": req.workflow_version,
                    "dataset_id": req.dataset_id,
                    "dataset_version": req.dataset_version,
                },
            )
        )
        return run

    def record_outcome(
        self,
        *,
        run_id: str,
        success: bool,
        cost_usd: float | None,
        latency_ms: float | None,
        business_metric_value: float | None,
    ) -> EvalRun:
        run = self.store.get_run(run_id)
        if run is None:
            raise ValueError("run_not_found")
        if run.completed_at is not None:
            raise ValueError("run_already_completed")
        # Update aggregates incrementally until all dataset items report an outcome.
        run.observations += 1
        run.failures += 0 if success else 1
        successes = run.observations - run.failures
        run.metric_value = (successes / run.observations) if run.observations else 0.0
        if cost_usd is not None:
            run.cost_usd = (run.cost_usd or 0.0) + cost_usd
        if latency_ms is not None:
            run.latency_p95_ms = max(run.latency_p95_ms or 0.0, latency_ms)
        if business_metric_value is not None:
            run.business_metric_value = (
                business_metric_value
                if run.business_metric_value is None
                else (run.business_metric_value + business_metric_value) / 2
            )
        if run.observations >= run.items:
            self._finalize(run)
        self.store.update_run(run)
        return run

    def _finalize(self, run: EvalRun) -> None:
        run.completed_at = datetime.now(timezone.utc)
        if run.baseline_value is not None and (run.baseline_value - run.metric_value) > run.delta_threshold:
            run.outcome = RunOutcome.REGRESSION_BLOCKED
            self.sink.emit(
                CloudEvent.new(
                    type_="workflow.publish.regression_blocked.v1",
                    tenant_id=run.tenant_id,
                    workspace_id=run.workspace_id,
                    subject=f"eval-run/{run.id}",
                    data={
                        "run_id": run.id,
                        "workflow_id": run.workflow_id,
                        "workflow_version": run.workflow_version,
                        "metric": run.metric_key,
                        "metric_value": run.metric_value,
                        "baseline_value": run.baseline_value,
                        "delta_threshold": run.delta_threshold,
                    },
                )
            )
        else:
            run.outcome = RunOutcome.PASSED
            self.sink.emit(
                CloudEvent.new(
                    type_="workflow.eval.run_completed.v1",
                    tenant_id=run.tenant_id,
                    workspace_id=run.workspace_id,
                    subject=f"eval-run/{run.id}",
                    data={
                        "run_id": run.id,
                        "workflow_id": run.workflow_id,
                        "workflow_version": run.workflow_version,
                        "metric": run.metric_key,
                        "metric_value": run.metric_value,
                    },
                )
            )

    # A/B --------------------------------------------------------------

    def start_ab(self, req: StartABRequest) -> ABRun:
        run = ABRun(
            tenant_id=req.tenant_id,
            workspace_id=req.workspace_id,
            workflow_id=req.workflow_id,
            version_a=req.version_a,
            version_b=req.version_b,
            target_executions=req.target_executions,
        )
        self.store.insert_ab(run)
        return run

    def record_ab_outcome(
        self,
        *,
        ab_run_id: str,
        variant: str,
        success: bool,
        cost_usd: float | None,
        latency_ms: float | None,
        business_metric_value: float | None,
    ) -> ABRun:
        run = self.store.get_ab(ab_run_id)
        if run is None:
            raise ValueError("ab_run_not_found")
        if variant not in ("a", "b"):
            raise ValueError("invalid_variant")
        run.counts[variant] = run.counts.get(variant, 0) + 1
        agg = run.metrics.setdefault(variant, {"successes": 0.0, "cost_usd": 0.0, "latency_ms": 0.0, "business": 0.0})
        agg["successes"] = agg.get("successes", 0.0) + (1.0 if success else 0.0)
        if cost_usd is not None:
            agg["cost_usd"] = agg.get("cost_usd", 0.0) + cost_usd
        if latency_ms is not None:
            agg["latency_ms"] = max(agg.get("latency_ms", 0.0), latency_ms)
        if business_metric_value is not None:
            agg["business"] = agg.get("business", 0.0) + business_metric_value
        if run.counts.get("a", 0) + run.counts.get("b", 0) >= run.target_executions:
            self._finalize_ab(run)
        self.store.update_ab(run)
        return run

    def _finalize_ab(self, run: ABRun) -> None:
        run.completed = True
        run.completed_at = datetime.now(timezone.utc)
        n_a = max(run.counts.get("a", 0), 1)
        n_b = max(run.counts.get("b", 0), 1)
        p_a = (run.metrics.get("a", {}).get("successes", 0.0)) / n_a
        p_b = (run.metrics.get("b", {}).get("successes", 0.0)) / n_b
        # Two-proportion z-test for significance at p<0.05.
        p_pool = (p_a * n_a + p_b * n_b) / (n_a + n_b)
        denom = math.sqrt(p_pool * (1 - p_pool) * (1 / n_a + 1 / n_b)) if 0 < p_pool < 1 else 0.0
        z = abs(p_a - p_b) / denom if denom > 0 else 0.0
        run.significant = z > 1.96
        self.sink.emit(
            CloudEvent.new(
                type_="workflow.eval.ab_completed.v1",
                tenant_id=run.tenant_id,
                workspace_id=run.workspace_id,
                subject=f"ab-run/{run.id}",
                data={
                    "ab_run_id": run.id,
                    "version_a": run.version_a,
                    "version_b": run.version_b,
                    "p_a": p_a,
                    "p_b": p_b,
                    "z": z,
                    "significant": run.significant,
                },
            )
        )

    # Reporting --------------------------------------------------------

    def is_publish_allowed(self, *, workflow_id: str, workflow_version: str) -> tuple[bool, dict[str, Any]]:
        """Returns (allowed, info). The runtime/registry consults this."""
        runs = [r for r in self.store.list_runs(workflow_id) if r.workflow_version == workflow_version]
        if not runs:
            return False, {"reason": "no_eval_run"}
        latest = runs[0]
        if latest.outcome == RunOutcome.REGRESSION_BLOCKED:
            return False, {"reason": "regression_detected", "run_id": latest.id, "metric": latest.metric_value}
        if latest.outcome == RunOutcome.PASSED:
            return True, {"run_id": latest.id, "metric": latest.metric_value}
        return False, {"reason": "run_failed", "run_id": latest.id}
