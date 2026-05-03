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

OUTPUT_SCHEMAS = {
    "create-user-stories": CREATE_USER_STORIES_OUTPUT,
    "scaffold-service": SCAFFOLD_SERVICE_OUTPUT,
    "generate-test-cases": GENERATE_TEST_CASES_OUTPUT,
}
