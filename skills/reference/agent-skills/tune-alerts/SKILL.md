---
name: tune-alerts
description: Tunes alerts against SLOs and operational signals. Use when alerts are too noisy, missing, or misaligned with error budgets.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-tune-alerts
  forge.tool_id: skill:tune-alerts
  forge.runtime: reference_skills.skills:tune_alerts
  forge.eval_suite: deterministic-reference-skills
---

# tune-alerts

Use this SDLC SRE skill to produce an alert tuning artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
