---
name: define-slos-from-spec
description: Defines SLOs from OpenSpec non-functional requirements. Use when reliability targets need SLIs, windows, and error budgets.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-define-slos-from-spec
  forge.tool_id: skill:define-slos-from-spec
  forge.runtime: reference_skills.skills:define_slos_from_spec
  forge.eval_suite: deterministic-reference-skills
---

# define-slos-from-spec

Use this SDLC SRE skill to produce SLO definitions linked to the OpenSpec.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
