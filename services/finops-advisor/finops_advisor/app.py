"""FastAPI app for the FinOps advisor."""

from __future__ import annotations

from fastapi import FastAPI

from .advisor import FinOpsAdvisor
from .events import LogSink, Sink
from .models import RecommendationsRequest


def create_app(advisor: FinOpsAdvisor | None = None, sink: Sink | None = None) -> FastAPI:
    if advisor is None:
        advisor = FinOpsAdvisor(sink=sink or LogSink())
    app = FastAPI(title="finops-advisor", version="0.1.0")
    app.state.advisor = advisor

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/finops/run")
    def run(req: RecommendationsRequest):
        return {"recommendations": app.state.advisor.run(req)}

    @app.get("/v1/finops/recommendations")
    def list_recs(tenant_id: str | None = None):
        return {
            "recommendations": app.state.advisor.list(tenant_id=tenant_id),
            "total_savings_usd_monthly": app.state.advisor.total_savings_usd_monthly(tenant_id=tenant_id),
            "by_kind": app.state.advisor.by_kind(tenant_id=tenant_id),
        }

    return app


app = create_app()
