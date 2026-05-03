from __future__ import annotations

import hashlib
from typing import Any


def create_user_stories(openspec: dict[str, Any]) -> dict[str, Any]:
    openspec_id = str(openspec.get("openspec_id") or "openspec")
    title = str(openspec.get("title") or "Forge initiative")
    requirements = (openspec.get("requirements") or {}).get("functional") or []
    epic_key = _key("EPIC", openspec_id, title)
    stories = []
    for index, requirement in enumerate(requirements, 1):
        key = _key("STORY", openspec_id, str(requirement))
        stories.append(
            {
                "key": key,
                "summary": str(requirement),
                "epic_key": epic_key,
                "acceptance_criteria": [f"Given {title}, when requirement {index} is implemented, then {requirement}"],
                "links": {"openspec_id": openspec_id, "direction": "bidirectional"},
            }
        )
    return {"epics": [{"key": epic_key, "summary": title, "openspec_id": openspec_id}], "stories": stories}


def scaffold_service(*, name: str, language: str = "python") -> dict[str, Any]:
    safe = name.replace("_", "-").lower()
    if language == "go":
        files = {
            "go.mod": f"module {safe}\n\ngo 1.22\n",
            "cmd/server/main.go": "package main\n\nfunc main() {}\n",
            "README.md": f"# {safe}\n",
        }
    else:
        package = safe.replace("-", "_")
        files = {
            "pyproject.toml": f"[project]\nname = \"{safe}\"\nversion = \"0.1.0\"\n",
            f"{package}/__init__.py": "",
            "README.md": f"# {safe}\n",
        }
    return {"language": language, "template": "forge-minimal-service", "files": files}


def generate_test_cases(openspec: dict[str, Any]) -> dict[str, Any]:
    requirements = (openspec.get("requirements") or {}).get("functional") or []
    cases = []
    for index, requirement in enumerate(requirements, 1):
        cases.append(
            {
                "id": f"TC-{index:03d}",
                "title": f"Validate: {requirement}",
                "type": "acceptance",
                "steps": ["Arrange workspace context", f"Execute behavior for: {requirement}", "Collect result"],
                "expected": str(requirement),
            }
        )
    return {"test_cases": cases}


def _key(prefix: str, *parts: str) -> str:
    digest = hashlib.sha1(":".join(parts).encode()).hexdigest()[:8].upper()
    return f"{prefix}-{digest}"
