"""Pydantic models for SDLC design skills."""

from __future__ import annotations

from enum import Enum
from typing import Any

from pydantic import BaseModel, Field


# ---------------------------------------------------------------------------
# generate-ui-blueprint
# ---------------------------------------------------------------------------


class UiBlueprintRequest(BaseModel):
    app_id: str
    openspec_id: str
    api_contract_path: str
    design_system_ref: str
    tenant_id: str
    workspace_id: str | None = None


class UiBlueprintResponse(BaseModel):
    blueprint_path: str
    figma_export: dict[str, Any] = Field(default_factory=dict)
    event_id: str


# ---------------------------------------------------------------------------
# generate-component-stubs
# ---------------------------------------------------------------------------


class ComponentStubsRequest(BaseModel):
    app_id: str
    blueprint_path: str
    design_system_ref: str
    framework: str = "react"  # react | vue
    tenant_id: str
    workspace_id: str | None = None


class StubFile(BaseModel):
    path: str
    content: str


class ComponentStubsResponse(BaseModel):
    stub_files: list[StubFile]
    event_id: str


# ---------------------------------------------------------------------------
# accessibility-audit
# ---------------------------------------------------------------------------


class AuditSeverity(str, Enum):
    critical = "critical"
    serious = "serious"
    moderate = "moderate"
    minor = "minor"


class AuditViolation(BaseModel):
    rule_id: str
    description: str
    severity: AuditSeverity
    target: str | None = None
    help_url: str | None = None


class AccessibilityAuditRequest(BaseModel):
    app_id: str
    blueprint_path: str
    stub_files: list[str] = Field(default_factory=list)
    tenant_id: str
    workspace_id: str | None = None


class AuditReport(BaseModel):
    total_violations: int
    violations: list[AuditViolation]
    summary_by_severity: dict[str, int]


class AccessibilityAuditResponse(BaseModel):
    audit_report: AuditReport
    audit_passed: bool  # False if any critical or serious violation exists
    event_id: str
