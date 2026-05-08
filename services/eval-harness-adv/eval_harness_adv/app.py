"""FastAPI app for the advanced eval harness."""

from __future__ import annotations

from fastapi import FastAPI, HTTPException

from .events import LogSink
from .models import (
    CreateDatasetRequest,
    EvalRun,
    RecordABOutcomeRequest,
    StartABRequest,
    StartRegressionRequest,
)
from .service import EvalHarness
from .store import Store


def create_app(harness: EvalHarness | None = None) -> FastAPI:
    if harness is None:
        harness = EvalHarness(store=Store(), sink=LogSink())
    app = FastAPI(title="eval-harness-adv", version="0.1.0")
    app.state.harness = harness

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/datasets")
    def create_dataset(req: CreateDatasetRequest):
        h: EvalHarness = app.state.harness
        try:
            return h.register_dataset(
                asset_id=req.asset_id,
                version=req.version,
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                description=req.description,
                trust_level=req.trust_level,
                items=req.items,
            )
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc

    @app.get("/v1/datasets/{asset_id}/versions")
    def list_dataset_versions(asset_id: str):
        h: EvalHarness = app.state.harness
        return {"versions": h.store.list_dataset_versions(asset_id)}

    @app.post("/v1/runs/regression")
    def start_regression(req: StartRegressionRequest):
        h: EvalHarness = app.state.harness
        try:
            return h.start_regression(req)
        except ValueError as exc:
            raise HTTPException(status_code=404, detail=str(exc)) from exc

    @app.post("/v1/runs/{run_id}/outcome")
    def record_outcome(
        run_id: str,
        success: bool,
        cost_usd: float | None = None,
        latency_ms: float | None = None,
        business_metric_value: float | None = None,
    ):
        h: EvalHarness = app.state.harness
        try:
            return h.record_outcome(
                run_id=run_id,
                success=success,
                cost_usd=cost_usd,
                latency_ms=latency_ms,
                business_metric_value=business_metric_value,
            )
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc

    @app.get("/v1/runs/{run_id}")
    def get_run(run_id: str) -> EvalRun:
        h: EvalHarness = app.state.harness
        run = h.store.get_run(run_id)
        if run is None:
            raise HTTPException(status_code=404, detail="run_not_found")
        return run

    @app.get("/v1/workflows/{workflow_id}/publish_allowed")
    def publish_allowed(workflow_id: str, version: str):
        h: EvalHarness = app.state.harness
        allowed, info = h.is_publish_allowed(workflow_id=workflow_id, workflow_version=version)
        return {"allowed": allowed, **info}

    @app.post("/v1/runs/ab")
    def start_ab(req: StartABRequest):
        h: EvalHarness = app.state.harness
        return h.start_ab(req)

    @app.post("/v1/runs/ab/outcome")
    def record_ab_outcome(req: RecordABOutcomeRequest):
        h: EvalHarness = app.state.harness
        try:
            return h.record_ab_outcome(
                ab_run_id=req.ab_run_id,
                variant=req.variant,
                success=req.success,
                cost_usd=req.cost_usd,
                latency_ms=req.latency_ms,
                business_metric_value=req.business_metric_value,
            )
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc

    return app


app = create_app()
