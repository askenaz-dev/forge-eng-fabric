---
name: update-threat-model
description: Updates a threat model after changes or findings. Use when security context changes and design evidence must stay current.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-update-threat-model
  forge.tool_id: skill:update-threat-model
  forge.runtime: reference_skills.skills:update_threat_model
  forge.eval_suite: deterministic-reference-skills
---

# update-threat-model

Use this SDLC security skill to produce a threat model update artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
