"""Pattern detectors for FinOps recommendations.

Each detector consumes a list of CostRecord entries and emits zero or more
Recommendation drafts. Detectors are intentionally pure: the orchestrator
(`FinOpsAdvisor`) is responsible for IDs, persistence and event emission.
"""

from __future__ import annotations

import uuid
from abc import ABC, abstractmethod
from collections import defaultdict
from typing import Any

from .models import CostRecord, Recommendation, RecommendationKind


class PatternDetector(ABC):
    name: str

    @abstractmethod
    def detect(self, records: list[CostRecord]) -> list[Recommendation]: ...


class IdleResourceDetector(PatternDetector):
    name = "idle"

    def __init__(self, threshold: float = 0.05, min_spend_usd: float = 5.0) -> None:
        self.threshold = threshold
        self.min_spend_usd = min_spend_usd

    def detect(self, records: list[CostRecord]) -> list[Recommendation]:
        out: list[Recommendation] = []
        for r in records:
            if r.kind != "cloud":
                continue
            if r.utilization is None:
                continue
            if r.utilization >= self.threshold or r.spend_usd < self.min_spend_usd:
                continue
            out.append(
                Recommendation(
                    id="rec-" + str(uuid.uuid4()),
                    tenant_id=r.tenant_id,
                    workspace_id=r.workspace_id,
                    asset_id=r.asset_id,
                    kind=RecommendationKind.IDLE_RESOURCE,
                    title=f"Idle resource {r.resource_id}",
                    detail=(
                        f"Resource {r.resource_id} has utilization "
                        f"{r.utilization:.2%} and spend ${r.spend_usd:.2f}/day."
                    ),
                    expected_savings_usd_monthly=r.spend_usd * 30 * 0.9,
                    affected_resources=[r.resource_id or ""],
                    severity="high" if r.spend_usd > 50 else "medium",
                    metadata={"utilization": r.utilization, "daily_spend": r.spend_usd},
                )
            )
        return out


class OversizedResourceDetector(PatternDetector):
    name = "oversized"

    def __init__(self, threshold: float = 0.30, min_spend_usd: float = 10.0) -> None:
        self.threshold = threshold
        self.min_spend_usd = min_spend_usd

    def detect(self, records: list[CostRecord]) -> list[Recommendation]:
        out: list[Recommendation] = []
        for r in records:
            if r.kind != "cloud":
                continue
            if r.utilization is None:
                continue
            if r.utilization >= self.threshold or r.spend_usd < self.min_spend_usd:
                continue
            # Skip those handled by IdleResourceDetector — utilization < 5%.
            if r.utilization < 0.05:
                continue
            out.append(
                Recommendation(
                    id="rec-" + str(uuid.uuid4()),
                    tenant_id=r.tenant_id,
                    workspace_id=r.workspace_id,
                    asset_id=r.asset_id,
                    kind=RecommendationKind.OVERSIZED_RESOURCE,
                    title=f"Oversized resource {r.resource_id}",
                    detail=(
                        f"Resource {r.resource_id} runs at {r.utilization:.2%} "
                        f"and costs ${r.spend_usd:.2f}/day. Consider downsizing."
                    ),
                    expected_savings_usd_monthly=r.spend_usd * 30 * 0.40,
                    affected_resources=[r.resource_id or ""],
                    severity="medium",
                    metadata={"utilization": r.utilization, "daily_spend": r.spend_usd},
                )
            )
        return out


class ExpensiveLLMSkillDetector(PatternDetector):
    name = "expensive_llm"

    def __init__(self, monthly_threshold_usd: float = 200.0) -> None:
        self.monthly_threshold_usd = monthly_threshold_usd

    def detect(self, records: list[CostRecord]) -> list[Recommendation]:
        spend_by_skill: dict[str, list[CostRecord]] = defaultdict(list)
        for r in records:
            if r.kind != "llm":
                continue
            if r.skill_id:
                spend_by_skill[r.skill_id].append(r)

        out: list[Recommendation] = []
        for skill_id, items in spend_by_skill.items():
            total_spend = sum(item.spend_usd for item in items)
            invocations = sum(item.invocations or 0 for item in items)
            if total_spend < self.monthly_threshold_usd:
                continue
            tenant_id = items[0].tenant_id
            workspace_id = items[0].workspace_id
            asset_id = items[0].asset_id
            avg_cost = total_spend / max(invocations, 1)
            out.append(
                Recommendation(
                    id="rec-" + str(uuid.uuid4()),
                    tenant_id=tenant_id,
                    workspace_id=workspace_id,
                    asset_id=asset_id,
                    kind=RecommendationKind.EXPENSIVE_LLM_SKILL,
                    title=f"Expensive skill {skill_id} (${total_spend:.0f}/period)",
                    detail=(
                        f"Skill {skill_id} cost ${total_spend:.2f} across "
                        f"{invocations} invocations (avg ${avg_cost:.4f}/call). "
                        f"Consider model downgrade or prompt simplification."
                    ),
                    expected_savings_usd_monthly=total_spend * 0.40,
                    affected_resources=[skill_id],
                    severity="high",
                    metadata={
                        "total_spend": total_spend,
                        "invocations": invocations,
                        "avg_cost_usd": avg_cost,
                    },
                )
            )
        return out


class CacheablePromptDetector(PatternDetector):
    name = "cacheable"

    def __init__(self, hit_rate_threshold: float = 0.10, min_invocations: int = 100) -> None:
        self.hit_rate_threshold = hit_rate_threshold
        self.min_invocations = min_invocations

    def detect(self, records: list[CostRecord]) -> list[Recommendation]:
        out: list[Recommendation] = []
        for r in records:
            if r.kind != "llm":
                continue
            if r.invocations is None or r.invocations < self.min_invocations:
                continue
            if r.cache_hit_rate is None:
                continue
            if r.cache_hit_rate >= self.hit_rate_threshold:
                continue
            out.append(
                Recommendation(
                    id="rec-" + str(uuid.uuid4()),
                    tenant_id=r.tenant_id,
                    workspace_id=r.workspace_id,
                    asset_id=r.asset_id,
                    kind=RecommendationKind.CACHEABLE_PROMPT,
                    title=f"Low cache hit-rate on {r.skill_id or r.asset_id}",
                    detail=(
                        f"Cache hit-rate is {r.cache_hit_rate:.2%} across "
                        f"{r.invocations} calls. Bump TTL / normalise prompts."
                    ),
                    expected_savings_usd_monthly=r.spend_usd * 30 * 0.20,
                    affected_resources=[r.skill_id or r.asset_id or ""],
                    severity="low",
                    metadata={
                        "cache_hit_rate": r.cache_hit_rate,
                        "invocations": r.invocations,
                    },
                )
            )
        return out


# --- BigQuery integration stub ---

class DailyCostQuery(ABC):
    """Wraps the BigQuery query that pulls cost rows for a given day.

    Production impl wires to `forge_finops.cost.daily_v1`. Tests pass an in-memory
    list directly into FinOpsAdvisor.run().
    """

    @abstractmethod
    def fetch(self, day: str) -> list[CostRecord]: ...


class StaticDailyCostQuery(DailyCostQuery):
    def __init__(self, records: list[CostRecord]) -> None:
        self._records = records

    def fetch(self, day: str) -> list[CostRecord]:
        _ = day  # noqa: ARG002
        return list(self._records)


def default_detectors() -> list[PatternDetector]:
    return [
        IdleResourceDetector(),
        OversizedResourceDetector(),
        ExpensiveLLMSkillDetector(),
        CacheablePromptDetector(),
    ]
