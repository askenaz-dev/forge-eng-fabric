from __future__ import annotations

from datetime import datetime, timezone

from postmortem.events import MemorySink
from postmortem.generator import REQUIRED_SECTIONS, PostmortemGenerator
from postmortem.models import (
    HealingActionRecord,
    PostmortemRequest,
    TimelineEvent,
)


def make_request(**overrides) -> PostmortemRequest:
    base = dict(
        incident_id="inc-1",
        tenant_id="t-1",
        workspace_id="w-1",
        asset_id="application/web/svc-foo",
        service="svc-foo",
        environment="prod",
        severity="critical",
        summary="5xx storm caused by stale cache",
        impact="3 minutes of degraded checkout",
        timeline=[
            TimelineEvent(occurred_at=datetime.now(timezone.utc), label="alert fired"),
            TimelineEvent(occurred_at=datetime.now(timezone.utc), label="cache refreshed"),
        ],
        healing_actions=[
            HealingActionRecord(action_id="refresh-cache", level="L4", outcome="executed"),
        ],
        diagnosis_summary="Stale Redis prefix returned 5xx for product listing",
        diagnosis_citations=[
            {"source_kind": "metric", "source_id": "prom:5xx-rate", "url": None}
        ],
        started_at=datetime.now(timezone.utc),
        resolved_at=datetime.now(timezone.utc),
    )
    base.update(overrides)
    return PostmortemRequest(**base)


def test_generate_renders_required_sections():
    sink = MemorySink()
    gen = PostmortemGenerator(sink=sink)
    draft = gen.generate(make_request())
    for section in REQUIRED_SECTIONS:
        assert section in draft.body_markdown
    assert any(e["type"] == "postmortem.generated.v1" for e in sink.events)


def test_eval_suite_flags_missing_owner():
    sink = MemorySink()
    gen = PostmortemGenerator(sink=sink)
    req = make_request()
    draft = gen.generate(req)
    # Force a bad owner.
    draft.action_items[0].owner = "@unknown"
    body_lines = draft.body_markdown.splitlines()
    # Replace the rendered owner line so the body matches the mutated draft.
    draft.body_markdown = "\n".join(
        line.replace("@sre-platform", "@unknown") if "@sre-platform" in line else line
        for line in body_lines
    )
    evaluation = gen.evaluate(draft, req)
    assert not evaluation["passed"]
    assert any("missing_owner" in failure for failure in evaluation["failures"])


def test_eval_suite_flags_missing_citation():
    sink = MemorySink()
    gen = PostmortemGenerator(sink=sink)
    # The default stub LLM puts diagnosis_summary in root_cause but does not
    # include citation source_ids unless the request does. A request whose
    # diagnosis_summary does not name the source_id will fail eval.
    req = make_request(
        diagnosis_summary="Stale Redis prefix returned 5xx for product listing",
        diagnosis_citations=[
            {"source_kind": "metric", "source_id": "definitely-not-in-body", "url": None}
        ],
    )
    draft = gen.generate(req)
    evaluation = gen.evaluate(draft, req)
    assert not evaluation["passed"]
    assert any(failure.startswith("missing_citation") for failure in evaluation["failures"])


def test_eval_suite_passes_when_summary_cites_evidence():
    sink = MemorySink()
    gen = PostmortemGenerator(sink=sink)
    req = make_request(
        diagnosis_summary=(
            "Stale Redis prefix prom:5xx-rate confirmed elevated 5xx rate for "
            "product listing"
        ),
        diagnosis_citations=[
            {"source_kind": "metric", "source_id": "prom:5xx-rate", "url": None}
        ],
    )
    draft = gen.generate(req)
    evaluation = gen.evaluate(draft, req)
    assert evaluation["passed"], evaluation["failures"]


def test_publish_calls_all_external_systems():
    sink = MemorySink()
    gen = PostmortemGenerator(sink=sink)
    req = make_request()
    draft = gen.generate(req)
    result = gen.publish(draft, req)
    assert result.confluence_url.startswith("confluence://")
    assert result.openspec_link.startswith("openspec://")
    assert result.jira_issue_keys, "expected at least one Jira issue"
    assert any(e["type"] == "postmortem.published.v1" for e in sink.events)


def test_synthetic_flag_propagates():
    sink = MemorySink()
    gen = PostmortemGenerator(sink=sink)
    req = make_request(synthetic=True)
    draft = gen.generate(req)
    gen.publish(draft, req)
    types = {e["type"]: e for e in sink.events}
    assert types["postmortem.generated.v1"]["data"]["synthetic"] is True
    assert types["postmortem.published.v1"]["data"]["synthetic"] is True
