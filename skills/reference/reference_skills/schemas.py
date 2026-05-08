from __future__ import annotations

CREATE_USER_STORIES_OUTPUT = {
    "type": "object",
    "required": ["epics", "stories"],
    "properties": {
        "epics": {"type": "array", "items": {"type": "object", "required": ["key", "summary"]}},
        "stories": {"type": "array", "items": {"type": "object", "required": ["key", "summary", "acceptance_criteria"]}},
    },
}

SCAFFOLD_SERVICE_OUTPUT = {
    "type": "object",
    "required": ["language", "files"],
    "properties": {
        "language": {"type": "string"},
        "files": {"type": "object", "additionalProperties": {"type": "string"}},
    },
}

GENERATE_TEST_CASES_OUTPUT = {
    "type": "object",
    "required": ["test_cases"],
    "properties": {
        "test_cases": {
            "type": "array",
            "items": {"type": "object", "required": ["id", "title", "type", "steps", "expected"]},
        }
    },
}

SDLC_SKILL_OUTPUT = {
    "type": "object",
    "required": ["skill", "capability", "artifacts", "recommendations", "links", "eval_score"],
    "properties": {
        "skill": {"type": "string"},
        "capability": {"type": "string"},
        "artifacts": {
            "type": "array",
            "items": {"type": "object", "required": ["id", "type", "title", "summary", "status"]},
        },
        "recommendations": {"type": "array", "items": {"type": "string"}},
        "links": {"type": "object", "required": ["openspec_id", "direction"]},
        "eval_score": {"type": "number"},
    },
}

OUTPUT_SCHEMAS = {
    "create-user-stories": CREATE_USER_STORIES_OUTPUT,
    "scaffold-service": SCAFFOLD_SERVICE_OUTPUT,
    "generate-test-cases": GENERATE_TEST_CASES_OUTPUT,
}

for _skill_name in [
    "refine-user-story",
    "generate-acceptance-criteria",
    "prioritize-backlog",
    "propose-adr",
    "evaluate-options",
    "check-openspec-alignment",
    "generate-api-contract",
    "propose-data-model",
    "lightweight-threat-model",
    "implement-feature-tests-first",
    "refactor-with-safety-net",
    "apply-code-review-feedback",
    "generate-test-plan",
    "generate-e2e-tests",
    "triage-test-failures",
    "triage-vuln",
    "propose-fix-for-finding",
    "update-threat-model",
    "prepare-release-notes",
    "validate-rollback-plan",
    "update-pipeline",
    "define-slos-from-spec",
    "generate-runbook",
    "tune-alerts",
    "estimate-cost-from-spec",
    "monitor-budget",
    "propose-cost-reduction",
]:
    OUTPUT_SCHEMAS[_skill_name] = SDLC_SKILL_OUTPUT
