---
name: apply-code-review-feedback
description: Structures code review feedback into actionable changes. Use when review comments need implementation planning and traceability.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-apply-code-review-feedback
  forge.tool_id: skill:apply-code-review-feedback
  forge.runtime: reference_skills.skills:apply_code_review_feedback
  forge.eval_suite: deterministic-reference-skills
---

# apply-code-review-feedback

Use this SDLC development skill to produce a deterministic review feedback plan.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
