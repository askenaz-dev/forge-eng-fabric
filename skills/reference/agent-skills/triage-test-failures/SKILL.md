---
name: triage-test-failures
description: Triage failing tests into likely causes and actions. Use when CI, E2E, or integration test runs fail.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-triage-test-failures
  forge.tool_id: skill:triage-test-failures
  forge.runtime: reference_skills.skills:triage_test_failures
  forge.eval_suite: deterministic-reference-skills
---

# triage-test-failures

Use this SDLC QA skill to create a deterministic failure triage artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
