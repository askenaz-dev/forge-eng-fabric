---
name: generate-e2e-tests
description: Generates E2E test suite guidance. Use when approved API contracts or user journeys need automated E2E coverage.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-generate-e2e-tests
  forge.tool_id: skill:generate-e2e-tests
  forge.runtime: reference_skills.skills:generate_e2e_tests
  forge.eval_suite: deterministic-reference-skills
---

# generate-e2e-tests

Use this SDLC QA skill to propose E2E tests linked to the OpenSpec.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
