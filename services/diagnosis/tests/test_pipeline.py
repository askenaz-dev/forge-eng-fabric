from __future__ import annotations

from diagnosis.events import MemorySink
from diagnosis.models import DiagnosisRequest
from diagnosis.pipeline import DiagnosisPipeline, StubLLM


def make_request(**overrides) -> DiagnosisRequest:
    base = dict(
        incident_id="inc-1",
        tenant_id="t-1",
        workspace_id="w-1",
        service="app-foo",
        environment="prod",
        signature_hash="abc123",
        severity="critical",
        title="HighErrorRate",
        description="5xx > 5%",
    )
    base.update(overrides)
    return DiagnosisRequest(**base)


def test_pipeline_emits_incident_diagnosed_event():
    sink = MemorySink()
    pipeline = DiagnosisPipeline(sink=sink)
    report = pipeline.run(make_request())
    assert report.prompt_version == "diagnose-incident@1.0.0"
    assert report.hypotheses, "expected at least one hypothesis"
    assert sink.events, "expected an event"
    assert sink.events[0]["type"] == "incident.diagnosed.v1"


def test_citation_enforcement_drops_unsupported_hypotheses():
    sink = MemorySink()
    fixtures = {
        "abc123": [
            {
                "statement": "fabricated",
                "confidence": 0.9,
                "rationale": "no source",
                "citations": [
                    {"source_kind": "metric", "source_id": "made-up-id"},
                ],
                "suggested_actions": [],
            },
            {
                "statement": "elevated 5xx caused by capacity",
                "confidence": 0.8,
                "rationale": "matches metric",
                # We have to know the source id the stub source emitted. The
                # PrometheusSource emits a deterministic id keyed by service+env.
                "citations": [
                    {
                        "source_kind": "metric",
                        "source_id": "prom:rate(http_requests_total{service='app-foo',env='prod',status=~'5..'}[5m])",
                    }
                ],
                "suggested_actions": ["scale-up"],
            },
        ]
    }
    pipeline = DiagnosisPipeline(sink=sink, llm=StubLLM(fixtures=fixtures))
    report = pipeline.run(make_request())
    statements = {h.statement for h in report.hypotheses}
    assert "fabricated" not in statements
    assert "elevated 5xx caused by capacity" in statements


def test_hypotheses_sorted_by_confidence():
    sink = MemorySink()
    # Use real metric ID present in PrometheusSource default output.
    metric_id = "prom:rate(http_requests_total{service='app-foo',env='prod',status=~'5..'}[5m])"
    fixtures = {
        "abc123": [
            {
                "statement": "low confidence",
                "confidence": 0.3,
                "citations": [{"source_kind": "metric", "source_id": metric_id}],
            },
            {
                "statement": "high confidence",
                "confidence": 0.9,
                "citations": [{"source_kind": "metric", "source_id": metric_id}],
            },
        ]
    }
    pipeline = DiagnosisPipeline(sink=sink, llm=StubLLM(fixtures=fixtures))
    report = pipeline.run(make_request())
    assert [h.statement for h in report.hypotheses] == ["high confidence", "low confidence"]


def test_synthetic_flag_propagates():
    sink = MemorySink()
    pipeline = DiagnosisPipeline(sink=sink)
    report = pipeline.run(make_request(synthetic=True))
    assert report.synthetic is True
    assert sink.events[0]["data"]["synthetic"] is True


def test_latency_is_recorded():
    sink = MemorySink()
    pipeline = DiagnosisPipeline(sink=sink)
    report = pipeline.run(make_request())
    assert report.duration_ms >= 0.0
    # The synthetic + stub pipeline finishes well under the 60-second p95 target.
    assert report.duration_ms < 1000.0
