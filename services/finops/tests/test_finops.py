from __future__ import annotations

from fastapi.testclient import TestClient

from finops_service.app import create_app
from finops_service.models import BillingExportRecord, Budget, LLMCostRecord
from finops_service.pipeline import create_budget, ingest_gcp_billing_export, ingest_llm_costs
from finops_service.store import FinOpsStore


def test_billing_export_requires_tags_and_emits_budget_alerts() -> None:
    store = FinOpsStore()
    create_budget(store, Budget(workspace_id="ws-1", initiative_openspec="spec-7", monthly_limit_usd=100.0))
    result = ingest_gcp_billing_export(
        store,
        [
            BillingExportRecord(
                service="Cloud Run",
                cost_usd=80.0,
                tags={"workspace": "ws-1", "env": "prod", "asset": "app-foo", "initiative_openspec": "spec-7"},
            )
        ],
    )
    assert result == {"records": 1, "cost_usd": 80.0}
    assert [event.threshold for event in store.events] == [50, 80]
    assert store.events[0].event_type == "finops.budget.threshold_reached.v1"


def test_llm_cost_import_aggregates_by_initiative() -> None:
    store = FinOpsStore()
    ingest_llm_costs(
        store,
        [
            LLMCostRecord(source="langfuse", workspace="ws-1", initiative_openspec="spec-7", model="gpt", cost_usd=1.25),
            LLMCostRecord(source="litellm", workspace="ws-1", initiative_openspec="spec-7", model="gpt", cost_usd=2.75),
        ],
    )
    assert store.costs_by_initiative() == {"spec-7": 4.0}


def test_finops_api_dashboard() -> None:
    store = FinOpsStore()
    client = TestClient(create_app(store))
    budget = client.post(
        "/v1/budgets",
        json={"workspace_id": "ws-1", "initiative_openspec": "spec-7", "monthly_limit_usd": 10.0},
    )
    assert budget.status_code == 200

    imported = client.post(
        "/v1/import/llm-costs",
        json=[{"source": "langfuse", "workspace": "ws-1", "initiative_openspec": "spec-7", "model": "gpt", "cost_usd": 5.0}],
    )
    assert imported.status_code == 200
    dashboard = client.get("/v1/dashboard")
    assert dashboard.json()["cost_by_initiative"] == {"spec-7": 5.0}
    assert dashboard.json()["events"][0]["threshold"] == 50
