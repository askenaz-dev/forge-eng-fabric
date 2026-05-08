# SDLC Gates

Gates are evaluated per phase before progression.

## Default Gates

- Product: `acceptance_criteria_present`, `story_size_estimated`
- Architecture: `adrs_published`, `security_review_passed`, `openspec_updated`
- Design: `api_contracts_defined`, `threat_model_present`, `data_model_documented`
- Development: `code_complete`, `lint_clean`, `unit_tests_passing`, `coverage`
- QA: `integration_tests_passing`, `e2e_tests_passing`, `perf_budget_met`
- Security: `sast_clean`, `sca_clean`, `dast_passed`, `secrets_clean`
- DevOps: `pipelines_green`, `image_signed`, `deploy_to_stage_successful`, `rollback_plan_present`
- SRE: `slos_defined`, `runbook_published`, `alerts_configured`, `on_call_assigned`
- FinOps: `cost_estimate_within_budget`, `llm_budget_within_limit`

## Thresholds

Coverage defaults by criticality:

- Low: 70%
- Medium: 75%
- High: 80%
- Critical: 85%

## Overrides

`phase-progression-bypass` requires:

- `release-manager` approval
- Mandatory rationale
- TTL at or below 24 hours
- Single-use consumption
- Full audit via `policy.override.*.v1` events
