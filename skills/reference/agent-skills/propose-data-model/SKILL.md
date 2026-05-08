---
name: propose-data-model
description: Proposes a data model for an initiative. Use when entities, relationships, or persistence concerns need a design artifact.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-propose-data-model
  forge.tool_id: skill:propose-data-model
  forge.runtime: reference_skills.skills:propose_data_model
  forge.eval_suite: deterministic-reference-skills
---

# propose-data-model

Use this SDLC design skill to produce a deterministic data model artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
