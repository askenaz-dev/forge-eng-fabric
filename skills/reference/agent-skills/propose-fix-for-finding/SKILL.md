---
name: propose-fix-for-finding
description: Proposes a fix for a security finding. Use when a vulnerability needs concrete remediation steps and test guidance.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-propose-fix-for-finding
  forge.tool_id: skill:propose-fix-for-finding
  forge.runtime: reference_skills.skills:propose_fix_for_finding
  forge.eval_suite: deterministic-reference-skills
---

# propose-fix-for-finding

Use this SDLC security skill to create a deterministic security fix plan.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
