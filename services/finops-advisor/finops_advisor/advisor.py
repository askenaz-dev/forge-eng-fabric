"""FinOps advisor orchestrator.

Runs daily (cron):
  1. Pulls cost data for the trailing day (BigQuery in production, fixture in tests).
  2. Runs every PatternDetector over the records.
  3. Persists recommendations and emits `finops.recommendation.created.v1`.
  4. The propose-cost-reduction skill drafts a PR for each recommendation.
     PRs follow Phase 2 + Phase 4 gates (the platform CI enforces this — the
     advisor only *opens* the PR; gating is owned by the existing pipeline).
"""

from __future__ import annotations

from collections import defaultdict
from datetime import datetime, timezone

from .events import Sink, new_event
from .models import CostRecord, Recommendation, RecommendationsRequest
from .patterns import PatternDetector, default_detectors
from .skills import GitHubPRClient, StubGitHubPR, open_pr_for


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


class FinOpsAdvisor:
    def __init__(
        self,
        sink: Sink,
        *,
        detectors: list[PatternDetector] | None = None,
        github: GitHubPRClient | None = None,
    ) -> None:
        self.sink = sink
        self.detectors = detectors if detectors is not None else default_detectors()
        self.github = github or StubGitHubPR()
        self._store: dict[str, Recommendation] = {}

    def run(self, req: RecommendationsRequest) -> list[Recommendation]:
        recs: list[Recommendation] = []
        for det in self.detectors:
            for rec in det.detect(req.records):
                rec.synthetic = req.synthetic
                self._store[rec.id] = rec
                recs.append(rec)
                self.sink.emit(
                    new_event(
                        tenant_id=req.tenant_id,
                        workspace_id=rec.workspace_id,
                        event_type="finops.recommendation.created.v1",
                        subject=f"recommendation/{rec.id}",
                        data={
                            "recommendation_id": rec.id,
                            "kind": rec.kind.value,
                            "expected_savings_usd_monthly": rec.expected_savings_usd_monthly,
                            "affected_resources": rec.affected_resources,
                            "synthetic": rec.synthetic,
                        },
                    )
                )
                open_pr_for(rec, self.github)
        return recs

    def list(self, tenant_id: str | None = None) -> list[Recommendation]:
        out = []
        for rec in self._store.values():
            if tenant_id and rec.tenant_id != tenant_id:
                continue
            out.append(rec)
        return out

    def total_savings_usd_monthly(self, tenant_id: str | None = None) -> float:
        total = 0.0
        for rec in self.list(tenant_id=tenant_id):
            total += rec.expected_savings_usd_monthly
        return total

    def by_kind(self, tenant_id: str | None = None) -> dict[str, int]:
        counts: dict[str, int] = defaultdict(int)
        for rec in self.list(tenant_id=tenant_id):
            counts[rec.kind.value] += 1
        return dict(counts)


def aggregate_records(records: list[CostRecord]) -> list[CostRecord]:
    """Aggregate by (tenant, skill_id|resource_id) so per-day rows roll up."""
    grouped: dict[tuple, list[CostRecord]] = defaultdict(list)
    for r in records:
        key = (r.tenant_id, r.skill_id or r.resource_id, r.kind)
        grouped[key].append(r)
    out: list[CostRecord] = []
    for (tenant_id, _, kind), items in grouped.items():
        first = items[0]
        spend = sum(i.spend_usd for i in items)
        invocations = sum(i.invocations or 0 for i in items) or None
        cache_hits = [i.cache_hit_rate for i in items if i.cache_hit_rate is not None]
        utils = [i.utilization for i in items if i.utilization is not None]
        out.append(
            CostRecord(
                tenant_id=tenant_id,
                workspace_id=first.workspace_id,
                asset_id=first.asset_id,
                service=first.service,
                skill_id=first.skill_id,
                resource_id=first.resource_id,
                kind=kind,
                spend_usd=spend,
                invocations=invocations,
                cache_hit_rate=(sum(cache_hits) / len(cache_hits)) if cache_hits else None,
                utilization=(sum(utils) / len(utils)) if utils else None,
                timestamp=first.timestamp,
            )
        )
    return out
