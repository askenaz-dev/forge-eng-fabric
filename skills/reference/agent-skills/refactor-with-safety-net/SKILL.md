---
name: refactor-with-safety-net
description: Plans a safe refactor backed by tests. Use when code should be changed without behavior regressions.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-refactor-with-safety-net
  forge.tool_id: skill:refactor-with-safety-net
  forge.runtime: reference_skills.skills:refactor_with_safety_net
  forge.eval_suite: deterministic-reference-skills
---

# refactor-with-safety-net

Use this SDLC development skill to create a refactor plan with a testing safety net.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
