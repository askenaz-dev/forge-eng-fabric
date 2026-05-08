---
name: generate-api-contract
description: Generates an API contract from design requirements. Use when an initiative needs OpenAPI-ready contract structure before development.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-generate-api-contract
  forge.tool_id: skill:generate-api-contract
  forge.runtime: reference_skills.skills:generate_api_contract
  forge.eval_suite: deterministic-reference-skills
---

# generate-api-contract

Use this SDLC design skill to produce an API contract artifact linked to the OpenSpec.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
