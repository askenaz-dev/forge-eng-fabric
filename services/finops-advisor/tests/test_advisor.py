from __future__ import annotations

from finops_advisor.advisor import FinOpsAdvisor
from finops_advisor.events import MemorySink
from finops_advisor.models import CostRecord, RecommendationKind, RecommendationsRequest


def cloud(resource_id: str, spend: float, util: float) -> CostRecord:
    return CostRecord(
        tenant_id="t-1",
        workspace_id="w-1",
        asset_id="application/foo",
        kind="cloud",
        resource_id=resource_id,
        spend_usd=spend,
        utilization=util,
    )


def llm(skill_id: str, spend: float, invocations: int, hit_rate: float | None = None) -> CostRecord:
    return CostRecord(
        tenant_id="t-1",
        workspace_id="w-1",
        kind="llm",
        skill_id=skill_id,
        spend_usd=spend,
        invocations=invocations,
        cache_hit_rate=hit_rate,
    )


def test_idle_resource_detected():
    sink = MemorySink()
    advisor = FinOpsAdvisor(sink=sink)
    recs = advisor.run(RecommendationsRequest(
        tenant_id="t-1",
        records=[cloud("vm-idle", spend=20, util=0.02)],
    ))
    kinds = {r.kind for r in recs}
    assert RecommendationKind.IDLE_RESOURCE in kinds
    assert any(e["type"] == "finops.recommendation.created.v1" for e in sink.events)


def test_oversized_resource_detected():
    sink = MemorySink()
    advisor = FinOpsAdvisor(sink=sink)
    recs = advisor.run(RecommendationsRequest(
        tenant_id="t-1",
        records=[cloud("vm-big", spend=100, util=0.18)],
    ))
    assert any(r.kind == RecommendationKind.OVERSIZED_RESOURCE for r in recs)


def test_expensive_llm_skill_detected():
    sink = MemorySink()
    advisor = FinOpsAdvisor(sink=sink)
    recs = advisor.run(RecommendationsRequest(
        tenant_id="t-1",
        records=[
            llm("skill/summarise", spend=210, invocations=4000),
            llm("skill/summarise", spend=120, invocations=2000),
        ],
    ))
    assert any(r.kind == RecommendationKind.EXPENSIVE_LLM_SKILL for r in recs)


def test_cacheable_prompt_detected_only_above_threshold():
    sink = MemorySink()
    advisor = FinOpsAdvisor(sink=sink)
    recs = advisor.run(RecommendationsRequest(
        tenant_id="t-1",
        records=[
            llm("skill/lookup", spend=20, invocations=500, hit_rate=0.05),
            # Below min_invocations threshold — should NOT trigger.
            llm("skill/rare", spend=5, invocations=10, hit_rate=0.0),
        ],
    ))
    cacheable = [r for r in recs if r.kind == RecommendationKind.CACHEABLE_PROMPT]
    assert len(cacheable) == 1
    assert "skill/lookup" in cacheable[0].affected_resources


def test_pr_created_for_each_recommendation():
    sink = MemorySink()
    advisor = FinOpsAdvisor(sink=sink)
    recs = advisor.run(RecommendationsRequest(
        tenant_id="t-1",
        records=[cloud("vm-idle", 50, 0.01), llm("skill/expensive", 300, 1000)],
    ))
    for r in recs:
        assert r.pr_url and r.pr_url.startswith("https://github.com/")
        assert r.pr_status == "open"


def test_total_savings_aggregates():
    sink = MemorySink()
    advisor = FinOpsAdvisor(sink=sink)
    advisor.run(RecommendationsRequest(
        tenant_id="t-1",
        records=[cloud("vm-idle", 50, 0.01), cloud("vm-big", 100, 0.20)],
    ))
    assert advisor.total_savings_usd_monthly("t-1") > 0


def test_synthetic_flag_propagates():
    sink = MemorySink()
    advisor = FinOpsAdvisor(sink=sink)
    recs = advisor.run(RecommendationsRequest(
        tenant_id="t-1",
        records=[cloud("vm-idle", 50, 0.01)],
        synthetic=True,
    ))
    assert all(r.synthetic for r in recs)
    assert sink.events[0]["data"]["synthetic"] is True
