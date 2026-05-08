---
name: prioritize-backlog
description: Prioritizes backlog items for product planning. Use when stories need ordering by impact, risk, and SDLC readiness.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-prioritize-backlog
  forge.tool_id: skill:prioritize-backlog
  forge.runtime: reference_skills.skills:prioritize_backlog
  forge.eval_suite: deterministic-reference-skills
---

# prioritize-backlog

Use this SDLC product skill to produce a deterministic backlog prioritization artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
