---
name: refine-user-story
description: Refines vague product input into a structured user story. Use when a Jira issue, OpenSpec requirement, or product note needs story shape and SDLC traceability.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-refine-user-story
  forge.tool_id: skill:refine-user-story
  forge.runtime: reference_skills.skills:refine_user_story
  forge.eval_suite: deterministic-reference-skills
---

# refine-user-story

Use this SDLC product skill to produce deterministic story refinement output linked to the OpenSpec.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
