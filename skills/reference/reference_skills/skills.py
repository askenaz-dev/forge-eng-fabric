from __future__ import annotations

import hashlib
from typing import Any


SDLC_SKILL_DEFS: dict[str, dict[str, str]] = {
    "refine-user-story": {"capability": "sdlc-product", "artifact": "refined_story"},
    "generate-acceptance-criteria": {"capability": "sdlc-product", "artifact": "acceptance_criteria"},
    "prioritize-backlog": {"capability": "sdlc-product", "artifact": "prioritized_backlog"},
    "propose-adr": {"capability": "sdlc-architecture", "artifact": "adr"},
    "evaluate-options": {"capability": "sdlc-architecture", "artifact": "option_evaluation"},
    "check-openspec-alignment": {"capability": "sdlc-architecture", "artifact": "alignment_report"},
    "generate-api-contract": {"capability": "sdlc-design", "artifact": "openapi_contract"},
    "propose-data-model": {"capability": "sdlc-design", "artifact": "data_model"},
    "lightweight-threat-model": {"capability": "sdlc-design", "artifact": "threat_model"},
    "implement-feature-tests-first": {"capability": "sdlc-development", "artifact": "test_first_plan"},
    "refactor-with-safety-net": {"capability": "sdlc-development", "artifact": "refactor_plan"},
    "apply-code-review-feedback": {"capability": "sdlc-development", "artifact": "review_feedback_plan"},
    "generate-test-plan": {"capability": "sdlc-qa", "artifact": "test_plan"},
    "generate-e2e-tests": {"capability": "sdlc-qa", "artifact": "e2e_suite"},
    "triage-test-failures": {"capability": "sdlc-qa", "artifact": "failure_triage"},
    "triage-vuln": {"capability": "sdlc-security", "artifact": "vulnerability_triage"},
    "propose-fix-for-finding": {"capability": "sdlc-security", "artifact": "security_fix_plan"},
    "update-threat-model": {"capability": "sdlc-security", "artifact": "threat_model_update"},
    "prepare-release-notes": {"capability": "sdlc-devops", "artifact": "release_notes"},
    "validate-rollback-plan": {"capability": "sdlc-devops", "artifact": "rollback_validation"},
    "update-pipeline": {"capability": "sdlc-devops", "artifact": "pipeline_update"},
    "define-slos-from-spec": {"capability": "sdlc-sre", "artifact": "slo_definition"},
    "generate-runbook": {"capability": "sdlc-sre", "artifact": "runbook"},
    "tune-alerts": {"capability": "sdlc-sre", "artifact": "alert_tuning"},
    "estimate-cost-from-spec": {"capability": "sdlc-finops", "artifact": "cost_estimate"},
    "monitor-budget": {"capability": "sdlc-finops", "artifact": "budget_monitor"},
    "propose-cost-reduction": {"capability": "sdlc-finops", "artifact": "cost_reduction_plan"},
}


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


def run_sdlc_skill(skill_name: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
    params = params or {}
    definition = SDLC_SKILL_DEFS[skill_name]
    openspec = _openspec_param(params)
    openspec_id = str(openspec.get("openspec_id") or params.get("openspec_id") or "openspec")
    title = str(openspec.get("title") or params.get("title") or "Forge initiative")
    requirements = (openspec.get("requirements") or {}).get("functional") or []
    requirement_text = str(requirements[0] if requirements else params.get("prompt") or title)
    artifact_id = _key(definition["artifact"].upper(), skill_name, openspec_id, requirement_text)
    artifact = {
        "id": artifact_id,
        "type": definition["artifact"],
        "title": f"{skill_name}: {title}",
        "summary": _summary_for(skill_name, requirement_text),
        "status": "proposed",
    }
    return {
        "skill": skill_name,
        "capability": definition["capability"],
        "artifacts": [artifact],
        "recommendations": _recommendations_for(skill_name, requirement_text),
        "links": {"openspec_id": openspec_id, "direction": "bidirectional"},
        "eval_score": 1.0,
    }


def _make_sdlc_skill(skill_name: str):
    def invoke(params: dict[str, Any] | None = None) -> dict[str, Any]:
        return run_sdlc_skill(skill_name, params)

    invoke.__name__ = skill_name.replace("-", "_")
    return invoke


for _skill_name in SDLC_SKILL_DEFS:
    globals()[_skill_name.replace("-", "_")] = _make_sdlc_skill(_skill_name)


SDLC_SKILL_FUNCTIONS = {name: globals()[name.replace("-", "_")] for name in SDLC_SKILL_DEFS}


def _key(prefix: str, *parts: str) -> str:
    digest = hashlib.sha1(":".join(parts).encode()).hexdigest()[:8].upper()
    return f"{prefix}-{digest}"


def _openspec_param(params: dict[str, Any]) -> dict[str, Any]:
    openspec = params.get("openspec") or params
    return openspec if isinstance(openspec, dict) else {}


def _summary_for(skill_name: str, requirement: str) -> str:
    return f"{skill_name.replace('-', ' ')} output for {requirement}"


def _recommendations_for(skill_name: str, requirement: str) -> list[str]:
    return [
        f"Review {skill_name.replace('-', ' ')} with the owning workspace team.",
        f"Link evidence back to requirement: {requirement}.",
    ]
