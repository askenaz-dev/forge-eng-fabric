---
name: generate-test-cases
description: Generates acceptance test cases from a Forge OpenSpec. Use when a user needs QA scenarios, acceptance tests, or structured validation cases from functional requirements.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-generate-test-cases
  forge.tool_id: skill:generate-test-cases
  forge.runtime: reference_skills.skills:generate_test_cases
  forge.eval_suite: deterministic-reference-skills
---

# generate-test-cases

Use this skill to derive deterministic acceptance test cases from a Forge OpenSpec.

## Inputs

- `openspec`: object containing `openspec_id`, `title`, and `requirements.functional`.

## Procedure

1. Read each functional requirement from the OpenSpec.
2. Create one acceptance test case per requirement.
3. Include arrange, execute, and collect-result steps.
4. Use stable case ids in requirement order.
5. Keep output deterministic for the same OpenSpec.

## Output

Return JSON with:

- `test_cases`: list of objects with `id`, `title`, `type`, `steps`, and `expected`.

## Forge Runtime

The reference runtime function is `reference_skills.skills:generate_test_cases`.
