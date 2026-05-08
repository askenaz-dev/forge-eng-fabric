---
name: lightweight-threat-model
description: Creates a lightweight threat model. Use when medium or higher criticality design work needs security risks captured before development.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-lightweight-threat-model
  forge.tool_id: skill:lightweight-threat-model
  forge.runtime: reference_skills.skills:lightweight_threat_model
  forge.eval_suite: deterministic-reference-skills
---

# lightweight-threat-model

Use this SDLC design skill to produce a concise threat model linked to the OpenSpec.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
