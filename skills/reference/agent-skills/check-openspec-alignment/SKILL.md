---
name: check-openspec-alignment
description: Checks architecture alignment with an OpenSpec. Use when ADRs, designs, or implementation plans need validation against stated requirements.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-check-openspec-alignment
  forge.tool_id: skill:check-openspec-alignment
  forge.runtime: reference_skills.skills:check_openspec_alignment
  forge.eval_suite: deterministic-reference-skills
---

# check-openspec-alignment

Use this SDLC architecture skill to report OpenSpec alignment findings.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
