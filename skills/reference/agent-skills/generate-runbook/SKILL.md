---
name: generate-runbook
description: Generates a runbook for an initiative. Use when operational procedures, diagnostics, and rollback steps need documentation.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-generate-runbook
  forge.tool_id: skill:generate-runbook
  forge.runtime: reference_skills.skills:generate_runbook
  forge.eval_suite: deterministic-reference-skills
---

# generate-runbook

Use this SDLC SRE skill to create a deterministic runbook artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
