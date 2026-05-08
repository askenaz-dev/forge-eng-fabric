---
name: update-pipeline
description: Proposes pipeline updates. Use when CI/CD configuration needs changes for gates, tests, deploys, or observability hooks.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-update-pipeline
  forge.tool_id: skill:update-pipeline
  forge.runtime: reference_skills.skills:update_pipeline
  forge.eval_suite: deterministic-reference-skills
---

# update-pipeline

Use this SDLC DevOps skill to produce a deterministic pipeline update artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
