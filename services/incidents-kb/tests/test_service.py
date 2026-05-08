from __future__ import annotations

from incidents_kb.events import MemorySink
from incidents_kb.models import IndexRequest, SimilarRequest
from incidents_kb.service import IncidentsKB


def make_request(incident_id: str, service: str = "app-foo", root_cause: str = "stale cache") -> IndexRequest:
    return IndexRequest(
        incident_id=incident_id,
        tenant_id="t-1",
        workspace_id="w-1",
        service=service,
        environment="prod",
        summary=f"5xx storm on {service}",
        symptoms="elevated 5xx with low latency",
        root_cause=root_cause,
        healing_actions=["refresh-cache"],
    )


def test_index_emits_event_and_stores_entry():
    sink = MemorySink()
    kb = IncidentsKB(sink=sink)
    entry = kb.index(make_request("inc-1"))
    assert entry.embedding, "expected an embedding"
    assert any(e["type"] == "kb.incident.indexed.v1" for e in sink.events)


def test_similar_returns_top_k_above_threshold():
    sink = MemorySink()
    kb = IncidentsKB(sink=sink)
    kb.index(make_request("inc-1", root_cause="redis stale cache caused 5xx"))
    kb.index(make_request("inc-2", root_cause="redis stale cache caused 5xx"))
    kb.index(make_request("inc-3", root_cause="completely unrelated config drift"))
    results = kb.similar(SimilarRequest(tenant_id="t-1", query="cache returning stale data 5xx", top_k=2))
    assert results, "expected matches"
    assert len(results) <= 2
    assert results[0].score >= results[-1].score


def test_recurrent_detection_emits_event():
    sink = MemorySink()
    kb = IncidentsKB(sink=sink)
    for i in range(3):
        kb.index(make_request(f"inc-{i}", root_cause="redis stale cache caused 5xx"))
    clusters = kb.detect_recurrent("t-1")
    assert clusters, "expected a cluster"
    assert any(e["type"] == "incident.recurrent.detected.v1" for e in sink.events)


def test_tenant_isolation():
    sink = MemorySink()
    kb = IncidentsKB(sink=sink)
    kb.index(make_request("inc-1"))
    other = SimilarRequest(tenant_id="t-other", query="anything", top_k=5)
    assert kb.similar(other) == []
