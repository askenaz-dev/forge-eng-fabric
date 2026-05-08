---
name: propose-adr
description: Proposes an Architecture Decision Record. Use when an initiative needs an ADR with rationale and OpenSpec linkage.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-propose-adr
  forge.tool_id: skill:propose-adr
  forge.runtime: reference_skills.skills:propose_adr
  forge.eval_suite: deterministic-reference-skills
---

# propose-adr

Use this SDLC architecture skill to propose an ADR artifact linked to the OpenSpec.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
