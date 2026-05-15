"""FastAPI application for sdlc-qa-skills.

Endpoints:
  POST /v1/skills/generate-test-plan
  POST /v1/skills/generate-e2e-tests
  POST /v1/skills/triage-test-failures
  POST /v1/hooks/ci-failed          (CloudEvent: ci.failed.v1)
  GET  /healthz
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any

from fastapi import FastAPI, HTTPException, Request, Response

from .events import LogSink, Sink
from .models import (
    CIFailedPayload,
    CIFailedResponse,
    GenerateE2ETestsRequest,
    GenerateE2ETestsResponse,
    GenerateTestPlanRequest,
    GenerateTestPlanResponse,
    TriageTestFailuresRequest,
    TriageTestFailuresResponse,
)
from .skills import LLMClient, QASkillRunner

logger = logging.getLogger("sdlc_qa_skills")

# ---------------------------------------------------------------------------
# Rate-limit cache: one triage report per PR per 10 minutes
# { pr_url: datetime }
# ---------------------------------------------------------------------------
_RATE_LIMIT_SECONDS = 600
_pr_last_triaged: dict[str, datetime] = {}


def _rate_limit_ok(pr_url: str) -> bool:
    now = datetime.now(timezone.utc)
    last = _pr_last_triaged.get(pr_url)
    if last is not None and (now - last).total_seconds() < _RATE_LIMIT_SECONDS:
        return False
    _pr_last_triaged[pr_url] = now
    return True


def _safety_eval_passes(result: Any) -> bool:
    """Minimal safety check before auto-opening a fix PR.

    In production this would call the eval-harness service. Here we require
    the proposed patch to be non-empty and all hypothesis confidences < 0.95
    (i.e. we are not blindly certain).
    """
    if not result.proposed_patch:
        return False
    if any(h.confidence >= 0.95 for h in result.top_hypotheses):
        return False
    return True


def _post_pr_comment(pr_url: str, body: str) -> None:
    """Stub: logs the comment. Replace with real GitHub/GitLab API call."""
    logger.info("PR comment on %s: %s", pr_url, body)


def _open_fix_pr(pr_url: str, patch: str) -> str:
    """Stub: returns a fake PR URL. Replace with real VCS API call."""
    logger.info("Auto-opening fix PR for %s with patch length %d", pr_url, len(patch))
    return pr_url.rstrip("/") + "/fix-auto"


# ---------------------------------------------------------------------------
# App factory
# ---------------------------------------------------------------------------


def create_app(runner: QASkillRunner | None = None, sink: Sink | None = None, llm: LLMClient | None = None) -> FastAPI:
    if runner is None:
        runner = QASkillRunner(sink=sink or LogSink(), llm=llm)
    app = FastAPI(title="sdlc-qa-skills", version="0.1.0")
    app.state.runner = runner

    # ------------------------------------------------------------------
    # Health
    # ------------------------------------------------------------------

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    # ------------------------------------------------------------------
    # Skills
    # ------------------------------------------------------------------

    @app.post("/v1/skills/generate-test-plan", response_model=GenerateTestPlanResponse)
    def generate_test_plan(req: GenerateTestPlanRequest) -> GenerateTestPlanResponse:
        r: QASkillRunner = app.state.runner
        try:
            return r.generate_test_plan(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/generate-e2e-tests", response_model=GenerateE2ETestsResponse)
    def generate_e2e_tests(req: GenerateE2ETestsRequest) -> GenerateE2ETestsResponse:
        r: QASkillRunner = app.state.runner
        try:
            return r.generate_e2e_tests(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/triage-test-failures", response_model=TriageTestFailuresResponse)
    def triage_test_failures(req: TriageTestFailuresRequest) -> TriageTestFailuresResponse:
        r: QASkillRunner = app.state.runner
        try:
            return r.triage_test_failures(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    # ------------------------------------------------------------------
    # Reactive hook: ci.failed.v1
    # ------------------------------------------------------------------

    @app.post("/v1/hooks/ci-failed", response_model=CIFailedResponse)
    def ci_failed(payload: CIFailedPayload) -> CIFailedResponse:
        r: QASkillRunner = app.state.runner

        # Rate-limit: one triage per PR per 10 minutes
        if not _rate_limit_ok(payload.pr_url):
            return CIFailedResponse(
                status="rate_limited",
                detail=f"Triage for {payload.pr_url} already dispatched within the last {_RATE_LIMIT_SECONDS}s",
            )

        try:
            triage_result = r.triage_test_failures(
                TriageTestFailuresRequest(
                    ci_run_id=payload.ci_run_id,
                    pr_url=payload.pr_url,
                )
            )
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

        # Post triage summary as PR comment
        top = triage_result.top_hypotheses[0].statement if triage_result.top_hypotheses else "unknown"
        comment = (
            f"**CI Triage** (run `{payload.ci_run_id}`)\n\n"
            f"Top hypothesis: {top}\n\n"
            f"Affected files: {', '.join(triage_result.affected_files) or 'none detected'}"
        )
        _post_pr_comment(payload.pr_url, comment)

        fix_pr_url: str | None = None
        qa_target = payload.targets.get("qa", "off")
        if qa_target in {"required", "autonomous"} and _safety_eval_passes(triage_result):
            fix_pr_url = _open_fix_pr(payload.pr_url, triage_result.proposed_patch or "")

        return CIFailedResponse(
            status="triaged",
            triage_event_id=triage_result.event_id,
            fix_pr_url=fix_pr_url,
        )

    return app


app = create_app()
