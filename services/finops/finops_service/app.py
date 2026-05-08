from __future__ import annotations

from fastapi import FastAPI

from finops_service.models import BillingExportRecord, Budget, LLMCostRecord
from finops_service.pipeline import create_budget, ingest_gcp_billing_export, ingest_llm_costs
from finops_service.store import FinOpsStore


def create_app(store: FinOpsStore | None = None) -> FastAPI:
    store = store or FinOpsStore()
    app = FastAPI(title="Forge FinOps Service", version="0.1.0")
    app.state.store = store

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/budgets", response_model=Budget)
    async def upsert_budget(budget: Budget) -> Budget:
        return create_budget(store, budget)

    @app.post("/v1/import/gcp-billing")
    async def import_gcp_billing(records: list[BillingExportRecord]) -> dict[str, int | float]:
        return ingest_gcp_billing_export(store, records)

    @app.post("/v1/import/llm-costs")
    async def import_llm_costs(records: list[LLMCostRecord]) -> dict[str, int | float]:
        return ingest_llm_costs(store, records)

    @app.get("/v1/dashboard")
    async def dashboard() -> dict[str, object]:
        return {
            "cost_by_initiative": store.costs_by_initiative(),
            "budgets": [budget.model_dump(mode="json") for budget in store.budgets.values()],
            "events": [event.model_dump(mode="json") for event in store.events],
        }

    return app


app = create_app()
