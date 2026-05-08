---
name: generate-test-plan
description: Generates a QA test plan. Use when an initiative needs unit, integration, E2E, contract, or performance coverage planning.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-generate-test-plan
  forge.tool_id: skill:generate-test-plan
  forge.runtime: reference_skills.skills:generate_test_plan
  forge.eval_suite: deterministic-reference-skills
---

# generate-test-plan

Use this SDLC QA skill to produce a deterministic test plan artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
