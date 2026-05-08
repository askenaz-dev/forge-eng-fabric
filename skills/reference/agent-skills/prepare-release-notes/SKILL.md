---
name: prepare-release-notes
description: Prepares release notes from linked changes. Use when deployment readiness needs user-facing and operator-facing release summaries.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-prepare-release-notes
  forge.tool_id: skill:prepare-release-notes
  forge.runtime: reference_skills.skills:prepare_release_notes
  forge.eval_suite: deterministic-reference-skills
---

# prepare-release-notes

Use this SDLC DevOps skill to produce deterministic release notes.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
