"""Skill implementations for the sdlc-qa-skills service.

All three skills are intentionally self-contained and use a stub LLM abstraction
so they work without an Anthropic API key in unit tests (swap in a real LLMClient
for production).
"""

from __future__ import annotations

import os
import re
import textwrap
import uuid
from abc import ABC, abstractmethod
from typing import Any

from .events import Sink, new_event
from .models import (
    GenerateE2ETestsRequest,
    GenerateE2ETestsResponse,
    GenerateTestPlanRequest,
    GenerateTestPlanResponse,
    Hypothesis,
    TriageTestFailuresRequest,
    TriageTestFailuresResponse,
)


# ---------------------------------------------------------------------------
# LLM abstraction
# ---------------------------------------------------------------------------


class LLMClient(ABC):
    @abstractmethod
    def complete(self, *, prompt: str, context: dict[str, Any]) -> dict[str, Any]: ...


class StubLLM(LLMClient):
    """Deterministic stub used in tests and offline flows."""

    def complete(self, *, prompt: str, context: dict[str, Any]) -> dict[str, Any]:
        task = context.get("task", "unknown")
        if task == "generate_test_plan":
            slug = context.get("spec_slug", "api")
            return {
                "test_plan": textwrap.dedent(f"""\
                    # Test Plan — {slug}

                    ## Scope
                    Generated from `{context.get('api_contract_path', '')}`.

                    ## Happy-path scenarios
                    - GET /health returns 200
                    - POST /items creates a resource and returns 201

                    ## Error scenarios
                    - Missing required fields -> 422
                    - Unauthorized request -> 401

                    ## Edge cases
                    - Empty list response → 200 with `[]`
                """)
            }
        if task == "generate_e2e_tests":
            slug = context.get("spec_slug", "api")
            return {
                "files": {
                    f"{slug}.spec.ts": textwrap.dedent(f"""\
                        import {{ test, expect }} from '@playwright/test';

                        test.describe('{slug} API', () => {{
                          test('health check', async ({{ request }}) => {{
                            const res = await request.get('/health');
                            expect(res.status()).toBe(200);
                          }});

                          test('create item', async ({{ request }}) => {{
                            const res = await request.post('/items', {{
                              data: {{ name: 'test' }},
                            }});
                            expect(res.status()).toBe(201);
                          }});
                        }});
                    """),
                    "playwright.config.ts": textwrap.dedent("""\
                        import { defineConfig } from '@playwright/test';
                        export default defineConfig({ testDir: '.' });
                    """),
                }
            }
        if task == "triage_failures":
            ci_run_id = context.get("ci_run_id", "")
            return {
                "top_hypotheses": [
                    {
                        "statement": "Flaky selector caused assertion failure",
                        "confidence": 0.82,
                        "rationale": f"Pattern observed in CI run {ci_run_id}",
                        "suggested_actions": ["stabilize-selector", "add-retry"],
                    },
                    {
                        "statement": "Race condition in async setup",
                        "confidence": 0.61,
                        "rationale": "Timing-sensitive test failed intermittently",
                        "suggested_actions": ["await-network-idle"],
                    },
                ],
                "affected_files": ["tests/e2e/items.spec.ts"],
                "proposed_patch": (
                    "--- a/tests/e2e/items.spec.ts\n"
                    "+++ b/tests/e2e/items.spec.ts\n"
                    "@@ -3,7 +3,8 @@\n"
                    "-  await page.click('#submit');\n"
                    "+  await page.locator('[data-testid=submit]').click();\n"
                    "+  await page.waitForResponse('**/items');\n"
                ),
            }
        return {}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _slugify(path: str) -> str:
    """Derive a filesystem-safe slug from a file path."""
    name = os.path.splitext(os.path.basename(path))[0]
    return re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-")


# ---------------------------------------------------------------------------
# Skill runner
# ---------------------------------------------------------------------------


class QASkillRunner:
    def __init__(self, *, sink: Sink, llm: LLMClient | None = None) -> None:
        self.sink = sink
        self.llm = llm or StubLLM()

    # ------------------------------------------------------------------
    # generate-test-plan
    # ------------------------------------------------------------------

    def generate_test_plan(self, req: GenerateTestPlanRequest) -> GenerateTestPlanResponse:
        spec_slug = _slugify(req.api_contract_path)
        result = self.llm.complete(
            prompt="generate_test_plan",
            context={
                "task": "generate_test_plan",
                "api_contract_path": req.api_contract_path,
                "spec_slug": spec_slug,
            },
        )
        test_plan_path = f"tests/plans/{spec_slug}.md"
        os.makedirs(os.path.dirname(test_plan_path), exist_ok=True)
        with open(test_plan_path, "w", encoding="utf-8") as fh:
            fh.write(result.get("test_plan", ""))

        event = new_event(
            tenant_id=req.tenant_id,
            workspace_id=req.workspace_id,
            event_type="sdlc.test_plan.proposed.v1",
            subject=f"spec/{spec_slug}",
            data={
                "api_contract_path": req.api_contract_path,
                "test_plan_path": test_plan_path,
                "spec_slug": spec_slug,
            },
        )
        self.sink.emit(event)
        return GenerateTestPlanResponse(
            test_plan_path=test_plan_path,
            spec_slug=spec_slug,
            event_id=event["id"],
        )

    # ------------------------------------------------------------------
    # generate-e2e-tests
    # ------------------------------------------------------------------

    def generate_e2e_tests(self, req: GenerateE2ETestsRequest) -> GenerateE2ETestsResponse:
        spec_slug = _slugify(req.test_plan_path)
        result = self.llm.complete(
            prompt="generate_e2e_tests",
            context={
                "task": "generate_e2e_tests",
                "test_plan_path": req.test_plan_path,
                "spec_slug": spec_slug,
            },
        )
        suite_dir = f"tests/e2e/{spec_slug}"
        os.makedirs(suite_dir, exist_ok=True)
        files: dict[str, str] = result.get("files", {})
        for filename, content in files.items():
            with open(os.path.join(suite_dir, filename), "w", encoding="utf-8") as fh:
                fh.write(content)

        return GenerateE2ETestsResponse(
            e2e_suite_path=suite_dir,
            spec_slug=spec_slug,
            file_count=len(files),
        )

    # ------------------------------------------------------------------
    # triage-test-failures
    # ------------------------------------------------------------------

    def triage_test_failures(self, req: TriageTestFailuresRequest) -> TriageTestFailuresResponse:
        result = self.llm.complete(
            prompt="triage_failures",
            context={
                "task": "triage_failures",
                "ci_run_id": req.ci_run_id,
                "pr_url": req.pr_url,
            },
        )
        hypotheses = [
            Hypothesis(
                statement=h["statement"],
                confidence=h["confidence"],
                rationale=h.get("rationale"),
                suggested_actions=h.get("suggested_actions", []),
            )
            for h in result.get("top_hypotheses", [])
        ]
        event = new_event(
            tenant_id=req.tenant_id,
            workspace_id=req.workspace_id,
            event_type="sdlc.test_failure.triaged.v1",
            subject=f"ci/{req.ci_run_id}",
            data={
                "ci_run_id": req.ci_run_id,
                "pr_url": req.pr_url,
                "hypothesis_count": len(hypotheses),
                "top_hypothesis": hypotheses[0].statement if hypotheses else "",
            },
        )
        self.sink.emit(event)
        return TriageTestFailuresResponse(
            ci_run_id=req.ci_run_id,
            top_hypotheses=hypotheses,
            affected_files=result.get("affected_files", []),
            proposed_patch=result.get("proposed_patch"),
            event_id=event["id"],
        )
