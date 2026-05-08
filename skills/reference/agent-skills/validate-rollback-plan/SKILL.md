---
name: validate-rollback-plan
description: Validates rollback readiness. Use when a release needs reversible deployment evidence before progression.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-validate-rollback-plan
  forge.tool_id: skill:validate-rollback-plan
  forge.runtime: reference_skills.skills:validate_rollback_plan
  forge.eval_suite: deterministic-reference-skills
---

# validate-rollback-plan

Use this SDLC DevOps skill to produce a rollback validation artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
