---
name: generate-acceptance-criteria
description: Generates acceptance criteria for product stories. Use when a requirement needs Given/When/Then-style validation before phase progression.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-generate-acceptance-criteria
  forge.tool_id: skill:generate-acceptance-criteria
  forge.runtime: reference_skills.skills:generate_acceptance_criteria
  forge.eval_suite: deterministic-reference-skills
---

# generate-acceptance-criteria

Use this SDLC product skill to create acceptance criteria linked to the OpenSpec.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
