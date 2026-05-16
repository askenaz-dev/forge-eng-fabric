"""Smoke tests for SDLC design skills."""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient

from sdlc_design_skills.app import create_app
from sdlc_design_skills.events import MemorySink
from sdlc_design_skills.models import (
    AccessibilityAuditRequest,
    ComponentStubsRequest,
    UiBlueprintRequest,
)
from sdlc_design_skills.skills import (
    accessibility_audit,
    generate_component_stubs,
    generate_ui_blueprint,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def make_blueprint_request(**overrides) -> UiBlueprintRequest:
    base = dict(
        app_id="app-1",
        openspec_id="spec-abc",
        api_contract_path="contracts/openapi/app-1.yaml",
        design_system_ref="forge-ds@1.0.0",
        tenant_id="t-1",
        workspace_id="w-1",
    )
    base.update(overrides)
    return UiBlueprintRequest(**base)


def make_stubs_request(**overrides) -> ComponentStubsRequest:
    base = dict(
        app_id="app-1",
        blueprint_path="artifacts/ui-blueprints/app-1/spec-abc/blueprint.json",
        design_system_ref="forge-ds@1.0.0",
        framework="react",
        tenant_id="t-1",
        workspace_id="w-1",
    )
    base.update(overrides)
    return ComponentStubsRequest(**base)


def make_audit_request(**overrides) -> AccessibilityAuditRequest:
    base = dict(
        app_id="app-1",
        blueprint_path="artifacts/ui-blueprints/app-1/spec-abc/blueprint.json",
        stub_files=["src/components/AppShell.tsx"],
        tenant_id="t-1",
        workspace_id="w-1",
    )
    base.update(overrides)
    return AccessibilityAuditRequest(**base)


# ---------------------------------------------------------------------------
# generate-ui-blueprint
# ---------------------------------------------------------------------------


def test_generate_ui_blueprint_returns_blueprint_path():
    sink = MemorySink()
    resp = generate_ui_blueprint(make_blueprint_request(), sink)
    assert resp.blueprint_path.startswith("artifacts/ui-blueprints/app-1/")
    assert resp.figma_export


def test_generate_ui_blueprint_emits_event():
    sink = MemorySink()
    resp = generate_ui_blueprint(make_blueprint_request(), sink)
    assert len(sink.events) == 1
    ev = sink.events[0]
    assert ev["type"] == "sdlc.ui_blueprint.proposed.v1"
    assert ev["id"] == resp.event_id
    assert ev["data"]["app_id"] == "app-1"


# ---------------------------------------------------------------------------
# generate-component-stubs
# ---------------------------------------------------------------------------


def test_generate_component_stubs_returns_files():
    sink = MemorySink()
    resp = generate_component_stubs(make_stubs_request(), sink)
    assert len(resp.stub_files) > 0
    for f in resp.stub_files:
        assert f.path.endswith(".tsx")
        assert "React" in f.content


def test_generate_component_stubs_vue_framework():
    sink = MemorySink()
    resp = generate_component_stubs(make_stubs_request(framework="vue"), sink)
    for f in resp.stub_files:
        assert f.path.endswith(".vue")
        assert "<template>" in f.content


def test_generate_component_stubs_emits_event():
    sink = MemorySink()
    resp = generate_component_stubs(make_stubs_request(), sink)
    assert len(sink.events) == 1
    ev = sink.events[0]
    assert ev["type"] == "sdlc.component_stubs.committed.v1"
    assert ev["id"] == resp.event_id
    assert ev["data"]["framework"] == "react"


# ---------------------------------------------------------------------------
# accessibility-audit
# ---------------------------------------------------------------------------


def test_accessibility_audit_returns_report():
    sink = MemorySink()
    resp = accessibility_audit(make_audit_request(), sink)
    assert resp.audit_report.total_violations > 0
    assert isinstance(resp.audit_passed, bool)


def test_accessibility_audit_fails_on_critical_or_serious():
    sink = MemorySink()
    resp = accessibility_audit(make_audit_request(), sink)
    # Placeholder data includes critical + serious violations
    assert resp.audit_passed is False


def test_accessibility_audit_summary_matches_violations():
    sink = MemorySink()
    resp = accessibility_audit(make_audit_request(), sink)
    report = resp.audit_report
    total_from_summary = sum(report.summary_by_severity.values())
    assert total_from_summary == report.total_violations


def test_accessibility_audit_emits_event():
    sink = MemorySink()
    resp = accessibility_audit(make_audit_request(), sink)
    assert len(sink.events) == 1
    ev = sink.events[0]
    assert ev["type"] == "sdlc.accessibility_audit.completed.v1"
    assert ev["id"] == resp.event_id
    assert ev["data"]["audit_passed"] is False


# ---------------------------------------------------------------------------
# HTTP layer (FastAPI TestClient)
# ---------------------------------------------------------------------------


@pytest.fixture()
def client():
    sink = MemorySink()
    return TestClient(create_app(sink=sink))


def test_healthz(client):
    r = client.get("/healthz")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_http_generate_ui_blueprint(client):
    payload = make_blueprint_request().model_dump()
    r = client.post("/v1/skills/generate-ui-blueprint", json=payload)
    assert r.status_code == 200
    body = r.json()
    assert "blueprint_path" in body
    assert "event_id" in body


def test_http_generate_component_stubs(client):
    payload = make_stubs_request().model_dump()
    r = client.post("/v1/skills/generate-component-stubs", json=payload)
    assert r.status_code == 200
    body = r.json()
    assert "stub_files" in body
    assert len(body["stub_files"]) > 0


def test_http_accessibility_audit(client):
    payload = make_audit_request().model_dump()
    r = client.post("/v1/skills/accessibility-audit", json=payload)
    assert r.status_code == 200
    body = r.json()
    assert "audit_report" in body
    assert "audit_passed" in body
    assert body["audit_passed"] is False


# --- alfred-design-system-picker (7.3) propagation tests ---


def test_ui_blueprint_event_carries_design_system_ref_top_level():
    """The workflow propagation contract requires sdlc.ui_blueprint.proposed.v1
    to carry design_system_ref at top level of `data` (not buried in metadata)
    so traceability-graph can read it without parsing the blueprint body."""
    sink = MemorySink()
    generate_ui_blueprint(make_blueprint_request(design_system_ref="desing-system-3@2.0.0"), sink)
    ev = sink.events[0]
    assert ev["data"]["design_system_ref"] == "desing-system-3@2.0.0", (
        f"missing top-level design_system_ref, got {ev['data']!r}"
    )


def test_component_stubs_event_carries_design_system_ref_top_level():
    sink = MemorySink()
    generate_component_stubs(make_stubs_request(design_system_ref="desing-system-3@2.0.0"), sink)
    ev = sink.events[0]
    assert ev["data"]["design_system_ref"] == "desing-system-3@2.0.0"


def test_accessibility_audit_event_carries_design_system_ref_top_level():
    sink = MemorySink()
    accessibility_audit(make_audit_request(design_system_ref="desing-system-3@2.0.0"), sink)
    ev = sink.events[0]
    assert ev["data"]["design_system_ref"] == "desing-system-3@2.0.0"


def test_accessibility_audit_accepts_omitted_design_system_ref_for_legacy_callers():
    """Pre-existing callers without the picker still work — design_system_ref
    is optional on AccessibilityAuditRequest. The event payload then carries
    None for that field, which downstream tools handle as 'unknown'."""
    sink = MemorySink()
    accessibility_audit(make_audit_request(), sink)
    ev = sink.events[0]
    # design_system_ref key MUST be present even when None for schema stability.
    assert "design_system_ref" in ev["data"]
    assert ev["data"]["design_system_ref"] is None
