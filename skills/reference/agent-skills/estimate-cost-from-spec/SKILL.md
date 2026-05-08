---
name: estimate-cost-from-spec
description: Estimates cost from OpenSpec requirements. Use when cloud, storage, network, or LLM spend needs forecast before completion.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-estimate-cost-from-spec
  forge.tool_id: skill:estimate-cost-from-spec
  forge.runtime: reference_skills.skills:estimate_cost_from_spec
  forge.eval_suite: deterministic-reference-skills
---

# estimate-cost-from-spec

Use this SDLC FinOps skill to produce a deterministic cost estimate artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
