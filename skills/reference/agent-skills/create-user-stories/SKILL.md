---
name: create-user-stories
description: Proposes Jira epics and user stories from a Forge OpenSpec. Use when a user asks to break requirements into backlog items, create Jira stories, or link product work to an OpenSpec.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-create-user-stories
  forge.tool_id: skill:create-user-stories
  forge.runtime: reference_skills.skills:create_user_stories
  forge.eval_suite: deterministic-reference-skills
---

# create-user-stories

Use this skill to turn a Forge OpenSpec into deterministic Jira-ready epics and user stories.

## Inputs

- `openspec`: object containing `openspec_id`, `title`, and `requirements.functional`.

## Procedure

1. Read the OpenSpec title and functional requirements.
2. Create one epic for the OpenSpec.
3. Create one story per functional requirement.
4. Include bidirectional links back to the OpenSpec in every story.
5. Keep generated keys deterministic for the same OpenSpec and requirement text.

## Output

Return JSON with:

- `epics`: list of epic objects with `key`, `summary`, and `openspec_id`.
- `stories`: list of story objects with `key`, `summary`, `epic_key`, `acceptance_criteria`, and `links`.

## Forge Runtime

The reference runtime function is `reference_skills.skills:create_user_stories`.
