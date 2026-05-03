---
name: scaffold-service
description: Creates a minimal Forge service scaffold. Use when a user needs a new service template, starter project files, or language-specific boilerplate for Python or Go.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-scaffold-service
  forge.tool_id: skill:scaffold-service
  forge.runtime: reference_skills.skills:scaffold_service
  forge.eval_suite: deterministic-reference-skills
---

# scaffold-service

Use this skill to generate a minimal service scaffold that can be committed or passed to later Forge onboarding flows.

## Inputs

- `name`: service name.
- `language`: target language. Supported values are `python` and `go`; default is `python`.

## Procedure

1. Normalize the service name to a safe package or module name.
2. Select the minimal Forge service template for the requested language.
3. Return generated files as an in-memory map; do not write to disk from the skill itself.
4. Keep output deterministic for the same inputs.

## Output

Return JSON with:

- `language`: selected language.
- `template`: template identifier.
- `files`: object where keys are file paths and values are file contents.

## Forge Runtime

The reference runtime function is `reference_skills.skills:scaffold_service`.
