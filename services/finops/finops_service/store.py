from __future__ import annotations

from dataclasses import dataclass, field

from finops_service.models import Budget, BudgetAlert, CostRecord


@dataclass
class FinOpsStore:
    budgets: dict[str, Budget] = field(default_factory=dict)
    costs: list[CostRecord] = field(default_factory=list)
    events: list[BudgetAlert] = field(default_factory=list)

    def upsert_budget(self, budget: Budget) -> Budget:
        key = budget_key(budget.workspace_id, budget.initiative_openspec)
        if key in self.budgets:
            existing = self.budgets[key]
            existing.monthly_limit_usd = budget.monthly_limit_usd
            existing.thresholds = budget.thresholds
            return existing
        self.budgets[key] = budget
        return budget

    def record_cost(self, cost: CostRecord) -> list[BudgetAlert]:
        self.costs.append(cost)
        budget = self.budgets.get(budget_key(cost.workspace_id, cost.initiative_openspec))
        if not budget:
            return []
        budget.consumed_usd += cost.cost_usd
        alerts: list[BudgetAlert] = []
        if budget.monthly_limit_usd <= 0:
            return alerts
        percent = (budget.consumed_usd / budget.monthly_limit_usd) * 100
        for threshold in sorted(budget.thresholds):
            if percent >= threshold and threshold not in budget.emitted_thresholds:
                budget.emitted_thresholds.add(threshold)
                alert = BudgetAlert(
                    workspace_id=budget.workspace_id,
                    initiative_openspec=budget.initiative_openspec,
                    threshold=threshold,
                    consumed_usd=budget.consumed_usd,
                    monthly_limit_usd=budget.monthly_limit_usd,
                    budget_id=budget.id,
                )
                self.events.append(alert)
                alerts.append(alert)
        return alerts

    def costs_by_initiative(self) -> dict[str, float]:
        totals: dict[str, float] = {}
        for cost in self.costs:
            totals[cost.initiative_openspec] = totals.get(cost.initiative_openspec, 0.0) + cost.cost_usd
        return totals


def budget_key(workspace_id: str, initiative_openspec: str) -> str:
    return f"{workspace_id}:{initiative_openspec}"
