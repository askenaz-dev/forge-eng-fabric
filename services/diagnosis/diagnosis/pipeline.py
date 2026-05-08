"""Diagnosis pipeline orchestrator.

Pipeline stages (D6.2):
  1. context-gather: pull metric / log / trace / OpenSpec / runbook / KB /
     eval / finops evidence.
  2. hypothesis-generation: a versioned prompt asks an LLM to produce ranked
     root-cause hypotheses with citations.
  3. citation-enforcement: drop any hypothesis whose citations do not match the
     evidence the system supplied.
  4. ranking: sort by confidence then KB-similarity match.
  5. emit: persist the diagnosis_report and emit incident.diagnosed.v1.

The LLM is abstracted behind the LLMClient protocol so tests can stub it.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from datetime import datetime, timezone
from typing import Any

from .events import Sink, new_event
from .models import (
    Citation,
    ContextBundle,
    DiagnosisReport,
    DiagnosisRequest,
    EvidenceBlock,
    Hypothesis,
)
from .prompts import DIAGNOSE_INCIDENT_PROMPT, DIAGNOSE_INCIDENT_PROMPT_VERSION
from .sources import EvidenceSource, default_sources


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


class LLMClient(ABC):
    @abstractmethod
    def complete(self, *, prompt: str, context: dict[str, Any]) -> dict[str, Any]: ...


class StubLLM(LLMClient):
    """Deterministic stub used in tests + offline / synthetic flows.

    Returns hypotheses keyed by signature_hash (or a default), citing the
    first evidence block of each kind from the context bundle. This keeps
    citation enforcement happy and makes unit tests stable.
    """

    def __init__(self, fixtures: dict[str, list[dict[str, Any]]] | None = None) -> None:
        self._fixtures = fixtures or {}

    def complete(self, *, prompt: str, context: dict[str, Any]) -> dict[str, Any]:
        sig = context.get("signature_hash", "")
        if sig in self._fixtures:
            hypotheses = self._fixtures[sig]
        else:
            evidence: list[dict[str, Any]] = context.get("evidence", [])
            citations = []
            for block in evidence[:2]:
                if block.get("citations"):
                    citations.append(block["citations"][0])
            hypotheses = [
                {
                    "statement": f"Service {context.get('service')} likely degraded",
                    "confidence": 0.7,
                    "rationale": "Composite signal from metrics + logs",
                    "citations": citations,
                    "suggested_actions": ["restart-pod"],
                }
            ]
        return {
            "context_summary": (
                f"Incident on {context.get('service')} in {context.get('environment')} — "
                f"signature {sig}."
            ),
            "hypotheses": hypotheses,
        }


class DiagnosisPipeline:
    """Coordinates context gathering, LLM call and citation enforcement."""

    def __init__(
        self,
        *,
        sink: Sink,
        sources: list[EvidenceSource] | None = None,
        llm: LLMClient | None = None,
        model_name: str = "stub",
    ) -> None:
        self.sink = sink
        self.sources = sources if sources is not None else default_sources()
        self.llm = llm or StubLLM()
        self.model_name = model_name

    def run(self, req: DiagnosisRequest) -> DiagnosisReport:
        started = utcnow()
        bundle = self._gather(req)
        raw = self.llm.complete(
            prompt=DIAGNOSE_INCIDENT_PROMPT,
            context=self._llm_context(bundle),
        )
        hypotheses = self._enforce_and_rank(bundle, raw.get("hypotheses", []))
        finished = utcnow()
        report = DiagnosisReport(
            incident_id=req.incident_id,
            prompt_version=DIAGNOSE_INCIDENT_PROMPT_VERSION,
            model=self.model_name,
            context_summary=raw.get("context_summary", ""),
            hypotheses=hypotheses,
            duration_ms=(finished - started).total_seconds() * 1000.0,
            started_at=started,
            finished_at=finished,
            synthetic=req.synthetic,
        )
        self.sink.emit(
            new_event(
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                event_type="incident.diagnosed.v1",
                subject=f"incident/{req.incident_id}",
                data={
                    "incident_id": req.incident_id,
                    "prompt_version": report.prompt_version,
                    "hypothesis_count": len(report.hypotheses),
                    "top_hypothesis": report.hypotheses[0].statement if report.hypotheses else "",
                    "duration_ms": report.duration_ms,
                    "synthetic": req.synthetic,
                },
            )
        )
        return report

    def _gather(self, req: DiagnosisRequest) -> ContextBundle:
        evidence: list[EvidenceBlock] = []
        for source in self.sources:
            try:
                evidence.extend(source.gather(req))
            except Exception:  # pragma: no cover — sources should not break pipeline
                continue
        return ContextBundle(request=req, evidence=evidence)

    def _llm_context(self, bundle: ContextBundle) -> dict[str, Any]:
        return {
            "service": bundle.request.service,
            "environment": bundle.request.environment,
            "signature_hash": bundle.request.signature_hash,
            "title": bundle.request.title,
            "description": bundle.request.description,
            "evidence": [b.model_dump() for b in bundle.evidence],
        }

    def _enforce_and_rank(
        self,
        bundle: ContextBundle,
        raw_hypotheses: list[dict[str, Any]],
    ) -> list[Hypothesis]:
        valid_ids = {c.source_id for b in bundle.evidence for c in b.citations}
        out: list[Hypothesis] = []
        for raw in raw_hypotheses:
            citations: list[Citation] = []
            for c in raw.get("citations", []):
                if isinstance(c, Citation):
                    citation = c
                else:
                    citation = Citation.model_validate(c)
                if citation.source_id in valid_ids:
                    citations.append(citation)
            if not citations:
                continue
            out.append(
                Hypothesis(
                    statement=raw.get("statement", ""),
                    confidence=float(raw.get("confidence", 0.0)),
                    rationale=raw.get("rationale"),
                    citations=citations,
                    suggested_actions=list(raw.get("suggested_actions", [])),
                )
            )
        out.sort(key=lambda h: h.confidence, reverse=True)
        return out
