"""Evidence-source adapters for the diagnosis pipeline.

Each source returns a small list of EvidenceBlocks with citations. Network
implementations live behind these interfaces; the in-memory variants below are
used in tests and synthetic flows.
"""

from __future__ import annotations

from abc import ABC, abstractmethod

from .models import Citation, DiagnosisRequest, EvidenceBlock


class EvidenceSource(ABC):
    name: str

    @abstractmethod
    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]: ...


class PrometheusSource(EvidenceSource):
    name = "prometheus"

    def __init__(self, fixtures: dict[str, list[EvidenceBlock]] | None = None) -> None:
        self._fixtures = fixtures or {}

    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]:
        # Stub: return preconfigured fixtures keyed by signature_hash if any.
        # Production implementation queries Prom for service+env error rates,
        # latency p95, CPU/mem saturation in the last 30 minutes.
        if req.signature_hash in self._fixtures:
            return self._fixtures[req.signature_hash]
        return [
            EvidenceBlock(
                kind="metric",
                summary=f"5xx rate for service={req.service} env={req.environment} elevated",
                citations=[
                    Citation(
                        source_kind="metric",
                        source_id=f"prom:rate(http_requests_total{{service='{req.service}',env='{req.environment}',status=~'5..'}}[5m])",
                        url=None,
                        excerpt=None,
                        score=0.9,
                    )
                ],
                raw=None,
            )
        ]


class LokiSource(EvidenceSource):
    name = "loki"

    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]:
        return [
            EvidenceBlock(
                kind="log",
                summary=f"Top error message for {req.service} in {req.environment}",
                citations=[
                    Citation(
                        source_kind="log",
                        source_id=f"loki:{{service=\"{req.service}\",env=\"{req.environment}\",level=\"error\"}}",
                    )
                ],
            )
        ]


class TempoSource(EvidenceSource):
    name = "tempo"

    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]:
        return [
            EvidenceBlock(
                kind="trace",
                summary=f"Slowest spans in last 15m for {req.service}",
                citations=[
                    Citation(
                        source_kind="trace",
                        source_id=f"tempo:service.name={req.service} env={req.environment}",
                    )
                ],
            )
        ]


class OpenSpecSource(EvidenceSource):
    name = "openspec"

    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]:
        return [
            EvidenceBlock(
                kind="openspec",
                summary=f"OpenSpec for {req.service} declares SLO p95 < 300ms",
                citations=[
                    Citation(
                        source_kind="openspec",
                        source_id=f"openspec:{req.service}",
                        url=f"openspec://services/{req.service}",
                    )
                ],
            )
        ]


class RunbookSource(EvidenceSource):
    name = "runbook"

    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]:
        return [
            EvidenceBlock(
                kind="runbook",
                summary=f"Runbook entry for symptom '{req.title}'",
                citations=[
                    Citation(
                        source_kind="runbook",
                        source_id=f"confluence://forge/runbooks/{req.service}",
                        url=f"confluence://forge/runbooks/{req.service}",
                    )
                ],
            )
        ]


class KBSource(EvidenceSource):
    name = "incidents-kb"

    def __init__(self, similar: list[dict] | None = None) -> None:
        self._similar = similar or []

    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]:
        if not self._similar:
            return []
        blocks = []
        for s in self._similar:
            blocks.append(
                EvidenceBlock(
                    kind="kb_incident",
                    summary=s.get("summary", ""),
                    citations=[
                        Citation(
                            source_kind="kb_incident",
                            source_id=s.get("incident_id", ""),
                            score=s.get("score"),
                        )
                    ],
                )
            )
        return blocks


class EvalSource(EvidenceSource):
    name = "evals"

    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]:
        return [
            EvidenceBlock(
                kind="eval",
                summary=f"Recent eval pass-rate for {req.service}",
                citations=[
                    Citation(
                        source_kind="eval",
                        source_id=f"eval:harness/{req.service}@latest",
                    )
                ],
            )
        ]


class FinOpsSource(EvidenceSource):
    name = "finops"

    def gather(self, req: DiagnosisRequest) -> list[EvidenceBlock]:
        return [
            EvidenceBlock(
                kind="finops",
                summary=f"Cost trend for {req.service} (last 24h)",
                citations=[
                    Citation(
                        source_kind="finops",
                        source_id=f"bigquery:cost.spend_by_service:{req.service}",
                    )
                ],
            )
        ]


def default_sources(*, kb_similar: list[dict] | None = None) -> list[EvidenceSource]:
    return [
        PrometheusSource(),
        LokiSource(),
        TempoSource(),
        OpenSpecSource(),
        RunbookSource(),
        KBSource(similar=kb_similar),
        EvalSource(),
        FinOpsSource(),
    ]
