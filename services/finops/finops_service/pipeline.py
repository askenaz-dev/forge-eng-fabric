from __future__ import annotations

from finops_service.models import BillingExportRecord, Budget, CostRecord, LLMCostRecord
from finops_service.store import FinOpsStore


def ingest_gcp_billing_export(store: FinOpsStore, records: list[BillingExportRecord]) -> dict[str, int | float]:
    total = 0.0
    for record in records:
        cost = CostRecord(
            workspace_id=record.tags["workspace"],
            initiative_openspec=record.tags["initiative_openspec"],
            env=record.tags["env"],
            asset=record.tags["asset"],
            category="cloud",
            source="gcp_billing_bigquery",
            cost_usd=record.cost_usd,
            metadata={"service": record.service, "currency": record.currency},
            observed_at=record.usage_start,
        )
        store.record_cost(cost)
        total += record.cost_usd
    return {"records": len(records), "cost_usd": total}


def ingest_llm_costs(store: FinOpsStore, records: list[LLMCostRecord]) -> dict[str, int | float]:
    total = 0.0
    for record in records:
        store.record_cost(
            CostRecord(
                workspace_id=record.workspace,
                initiative_openspec=record.initiative_openspec,
                category="llm",
                source=record.source,
                cost_usd=record.cost_usd,
                metadata={"model": record.model, "tokens": record.tokens},
                observed_at=record.observed_at,
            )
        )
        total += record.cost_usd
    return {"records": len(records), "cost_usd": total}


def create_budget(store: FinOpsStore, budget: Budget) -> Budget:
    return store.upsert_budget(budget)
