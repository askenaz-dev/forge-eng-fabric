"""Skill implementations for SDLC design skills."""

from __future__ import annotations

import uuid
from typing import Any

from .events import EventSink, LogSink, new_event
from .models import (
    AccessibilityAuditRequest,
    AccessibilityAuditResponse,
    AuditReport,
    AuditSeverity,
    AuditViolation,
    ComponentStubsRequest,
    ComponentStubsResponse,
    StubFile,
    UiBlueprintRequest,
    UiBlueprintResponse,
)

# ---------------------------------------------------------------------------
# generate-ui-blueprint
# ---------------------------------------------------------------------------

_FIGMA_EXPORT_TEMPLATE: dict[str, Any] = {
    "version": "1.0",
    "document": {
        "type": "DOCUMENT",
        "children": [
            {
                "type": "CANVAS",
                "name": "Page 1",
                "children": [
                    {
                        "type": "FRAME",
                        "name": "Main",
                        "width": 1440,
                        "height": 900,
                        "children": [],
                    }
                ],
            }
        ],
    },
}


def generate_ui_blueprint(req: UiBlueprintRequest, sink: EventSink) -> UiBlueprintResponse:
    """Produce a Figma-export JSON blueprint from an API contract + design-system ref."""
    blueprint_path = (
        f"artifacts/ui-blueprints/{req.app_id}/{req.openspec_id}/blueprint.json"
    )
    figma_export = dict(_FIGMA_EXPORT_TEMPLATE)
    figma_export["metadata"] = {
        "app_id": req.app_id,
        "openspec_id": req.openspec_id,
        "api_contract_path": req.api_contract_path,
        "design_system_ref": req.design_system_ref,
    }

    event = new_event(
        tenant_id=req.tenant_id,
        workspace_id=req.workspace_id,
        event_type="sdlc.ui_blueprint.proposed.v1",
        subject=f"app/{req.app_id}/blueprint",
        data={
            "app_id": req.app_id,
            "openspec_id": req.openspec_id,
            "blueprint_path": blueprint_path,
            # alfred-design-system-picker (7.3): top-level so downstream
            # subscribers (traceability-graph, observability) can read the
            # ref without parsing the blueprint document body.
            "design_system_ref": req.design_system_ref,
        },
    )
    sink.emit(event)

    return UiBlueprintResponse(
        blueprint_path=blueprint_path,
        figma_export=figma_export,
        event_id=event["id"],
    )


# ---------------------------------------------------------------------------
# generate-component-stubs
# ---------------------------------------------------------------------------

_REACT_STUB_TEMPLATE = """\
import React from 'react';
// Design System tokens from {design_system_ref}
// Blueprint source: {blueprint_path}

interface {component_name}Props {{
  // TODO: define props
}}

export const {component_name}: React.FC<{component_name}Props> = (props) => {{
  return <div className="{css_class}">{{}}</div>;
}};

export default {component_name};
"""

_VUE_STUB_TEMPLATE = """\
<template>
  <!-- Design System tokens from {design_system_ref} -->
  <!-- Blueprint source: {blueprint_path} -->
  <div class="{css_class}">
    <!-- TODO: implement component -->
  </div>
</template>

<script lang="ts">
import {{ defineComponent }} from 'vue';

export default defineComponent({{
  name: '{component_name}',
  props: {{
    // TODO: define props
  }},
}});
</script>
"""


def _make_stub(
    component_name: str,
    blueprint_path: str,
    design_system_ref: str,
    framework: str,
) -> StubFile:
    css_class = component_name.lower().replace(" ", "-")
    base_dir = "src/components"
    if framework == "vue":
        content = _VUE_STUB_TEMPLATE.format(
            component_name=component_name,
            blueprint_path=blueprint_path,
            design_system_ref=design_system_ref,
            css_class=css_class,
        )
        path = f"{base_dir}/{component_name}.vue"
    else:
        content = _REACT_STUB_TEMPLATE.format(
            component_name=component_name,
            blueprint_path=blueprint_path,
            design_system_ref=design_system_ref,
            css_class=css_class,
        )
        path = f"{base_dir}/{component_name}.tsx"
    return StubFile(path=path, content=content)


# Placeholder component list — real impl would parse the blueprint
_PLACEHOLDER_COMPONENTS = ["AppShell", "NavBar", "ContentArea", "Footer"]


def generate_component_stubs(req: ComponentStubsRequest, sink: EventSink) -> ComponentStubsResponse:
    """Produce React/Vue component stubs using Design System tokens."""
    stub_files = [
        _make_stub(name, req.blueprint_path, req.design_system_ref, req.framework)
        for name in _PLACEHOLDER_COMPONENTS
    ]

    event = new_event(
        tenant_id=req.tenant_id,
        workspace_id=req.workspace_id,
        event_type="sdlc.component_stubs.committed.v1",
        subject=f"app/{req.app_id}/stubs",
        data={
            "app_id": req.app_id,
            "blueprint_path": req.blueprint_path,
            "framework": req.framework,
            "stub_count": len(stub_files),
            "stub_paths": [f.path for f in stub_files],
            # alfred-design-system-picker (7.3): top-level so downstream
            # subscribers can link stubs → DS without inspecting file bodies.
            "design_system_ref": req.design_system_ref,
        },
    )
    sink.emit(event)

    return ComponentStubsResponse(stub_files=stub_files, event_id=event["id"])


# ---------------------------------------------------------------------------
# accessibility-audit
# ---------------------------------------------------------------------------

# Placeholder violations — real impl would run Axe-core against rendered HTML
_PLACEHOLDER_VIOLATIONS: list[dict] = [
    {
        "rule_id": "color-contrast",
        "description": "Elements must have sufficient color contrast",
        "severity": "serious",
        "target": "button.primary",
        "help_url": "https://dequeuniversity.com/rules/axe/4.9/color-contrast",
    },
    {
        "rule_id": "image-alt",
        "description": "Images must have alternative text",
        "severity": "critical",
        "target": "img.hero",
        "help_url": "https://dequeuniversity.com/rules/axe/4.9/image-alt",
    },
    {
        "rule_id": "label",
        "description": "Form elements must have labels",
        "severity": "moderate",
        "target": "input#email",
        "help_url": "https://dequeuniversity.com/rules/axe/4.9/label",
    },
]

_BLOCKING_SEVERITIES = {AuditSeverity.critical, AuditSeverity.serious}


def accessibility_audit(req: AccessibilityAuditRequest, sink: EventSink) -> AccessibilityAuditResponse:
    """Run Axe-core style analysis and classify violations by severity."""
    violations = [AuditViolation(**v) for v in _PLACEHOLDER_VIOLATIONS]

    summary: dict[str, int] = {s.value: 0 for s in AuditSeverity}
    for v in violations:
        summary[v.severity.value] += 1

    audit_passed = not any(v.severity in _BLOCKING_SEVERITIES for v in violations)
    audit_report = AuditReport(
        total_violations=len(violations),
        violations=violations,
        summary_by_severity=summary,
    )

    event = new_event(
        tenant_id=req.tenant_id,
        workspace_id=req.workspace_id,
        event_type="sdlc.accessibility_audit.completed.v1",
        subject=f"app/{req.app_id}/audit",
        data={
            "app_id": req.app_id,
            "blueprint_path": req.blueprint_path,
            "audit_passed": audit_passed,
            "total_violations": audit_report.total_violations,
            "summary_by_severity": summary,
            # alfred-design-system-picker (7.3): top-level so the audit can
            # be correlated with the App's resolved DS at dispatch time.
            "design_system_ref": req.design_system_ref,
        },
    )
    sink.emit(event)

    return AccessibilityAuditResponse(
        audit_report=audit_report,
        audit_passed=audit_passed,
        event_id=event["id"],
    )
