---
name: evaluate-options
description: Evaluates architecture options. Use when trade-offs need structured comparison before design progression.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-evaluate-options
  forge.tool_id: skill:evaluate-options
  forge.runtime: reference_skills.skills:evaluate_options
  forge.eval_suite: deterministic-reference-skills
---

# evaluate-options

Use this SDLC architecture skill to create deterministic option evaluation output.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
