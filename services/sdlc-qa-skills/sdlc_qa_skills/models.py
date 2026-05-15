"""Pydantic models for the sdlc-qa-skills service."""

from __future__ import annotations

from pydantic import BaseModel, Field


# ---------------------------------------------------------------------------
# generate-test-plan
# ---------------------------------------------------------------------------


class GenerateTestPlanRequest(BaseModel):
    api_contract_path: str
    """Path to the OpenAPI contract file (e.g. contracts/openapi/my-service.yaml)."""
    tenant_id: str | None = None
    workspace_id: str | None = None


class GenerateTestPlanResponse(BaseModel):
    test_plan_path: str
    """Absolute or repo-relative path where the markdown test plan was written."""
    spec_slug: str
    event_id: str


# ---------------------------------------------------------------------------
# generate-e2e-tests
# ---------------------------------------------------------------------------


class GenerateE2ETestsRequest(BaseModel):
    test_plan_path: str
    """Path to a previously generated test plan markdown file."""
    tenant_id: str | None = None
    workspace_id: str | None = None


class GenerateE2ETestsResponse(BaseModel):
    e2e_suite_path: str
    """Directory path where the Playwright test suite was written."""
    spec_slug: str
    file_count: int


# ---------------------------------------------------------------------------
# triage-test-failures
# ---------------------------------------------------------------------------


class TriageTestFailuresRequest(BaseModel):
    ci_run_id: str
    pr_url: str
    tenant_id: str | None = None
    workspace_id: str | None = None


class Hypothesis(BaseModel):
    statement: str
    confidence: float
    rationale: str | None = None
    suggested_actions: list[str] = Field(default_factory=list)


class TriageTestFailuresResponse(BaseModel):
    ci_run_id: str
    top_hypotheses: list[Hypothesis]
    affected_files: list[str]
    proposed_patch: str | None = None
    event_id: str


# ---------------------------------------------------------------------------
# ci-failed hook (CloudEvent body)
# ---------------------------------------------------------------------------


class CIFailedPayload(BaseModel):
    """Body of the ci.failed.v1 CloudEvent."""

    ci_run_id: str
    pr_url: str
    app_id: str
    targets: dict[str, str] = Field(default_factory=dict)
    """targets.qa can be 'required' | 'autonomous' | 'off'"""


class CIFailedResponse(BaseModel):
    status: str
    triage_event_id: str | None = None
    fix_pr_url: str | None = None
    detail: str | None = None
