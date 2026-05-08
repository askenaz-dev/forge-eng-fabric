---
name: propose-cost-reduction
description: Proposes cost reduction actions. Use when cloud or LLM spend exceeds budget and remediation options are needed.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-propose-cost-reduction
  forge.tool_id: skill:propose-cost-reduction
  forge.runtime: reference_skills.skills:propose_cost_reduction
  forge.eval_suite: deterministic-reference-skills
---

# propose-cost-reduction

Use this SDLC FinOps skill to produce a deterministic cost reduction plan.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
