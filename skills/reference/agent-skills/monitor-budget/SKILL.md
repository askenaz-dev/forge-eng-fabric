---
name: monitor-budget
description: Monitors budget consumption for an initiative. Use when spend thresholds, burn rate, or budget events need analysis.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-monitor-budget
  forge.tool_id: skill:monitor-budget
  forge.runtime: reference_skills.skills:monitor_budget
  forge.eval_suite: deterministic-reference-skills
---

# monitor-budget

Use this SDLC FinOps skill to produce a budget monitoring artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
