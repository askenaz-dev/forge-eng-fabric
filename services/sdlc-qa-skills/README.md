# sdlc-qa-skills

FastAPI skill service for the SDLC QA phase. Port: **8106**.

## Skills

| Skill | Route | Description |
|-------|-------|-------------|
| `generate-test-plan` | `POST /v1/skills/generate-test-plan` | API contract coverage + happy/error paths + perf cases |
| `generate-e2e-tests` | `POST /v1/skills/generate-e2e-tests` | Playwright suite from test plan |
| `triage-test-failures` | `POST /v1/skills/triage-test-failures` | Hypotheses + minimal patch proposal |
| CI hook | `POST /v1/hooks/ci-failed` | Reactive triage on `ci.failed.v1` events |

## Reactive CI hook

The CI hook rate-limits to **one report per PR per 10 minutes**. Auto-fix PRs are only opened when `targets.qa ∈ {required}` and the safety eval passes (≤200 lines diff, no protected paths, no secret references).

## Gates wired

`integration_tests_passing`, `e2e_tests_passing`, `perf_budget_met` (criticality ≥ high)

## Eval baseline

T1 promotion requires ≥30 graded fixtures per skill. Adversarial fixtures for `triage-test-failures`: flaky test mis-classification, large diffs that should be downgraded, secret-containing patches.

## Running locally

```bash
cd services/sdlc-qa-skills
uv run --extra dev uvicorn sdlc_qa_skills.app:app --reload --port 8106
uv run --extra dev pytest -q
```
