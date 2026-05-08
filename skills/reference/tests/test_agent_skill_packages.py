from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
AGENT_SKILLS_DIR = ROOT / "agent-skills"
REGISTRY_ASSETS = ROOT / "registry-assets.yaml"
SKILL_NAME = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


def test_agent_skill_packages_follow_core_spec() -> None:
    skill_dirs = sorted(path for path in AGENT_SKILLS_DIR.iterdir() if path.is_dir())
    assert {path.name for path in skill_dirs}.issuperset({
        "create-user-stories",
        "generate-test-cases",
        "scaffold-service",
    })

    for skill_dir in skill_dirs:
        frontmatter, body = _read_skill(skill_dir / "SKILL.md")

        name = frontmatter["name"]
        description = frontmatter["description"]
        assert name == skill_dir.name
        assert SKILL_NAME.match(name)
        assert 1 <= len(name) <= 64
        assert 1 <= len(description) <= 1024
        assert "Use when" in description
        assert len(body.strip()) > 0

        metadata = frontmatter["metadata"]
        assert metadata["forge.asset_type"] == "skill"
        assert metadata["forge.tool_id"] == f"skill:{name}"
        assert metadata["forge.runtime"].startswith("reference_skills.skills:")
        assert metadata["forge.eval_suite"] == "deterministic-reference-skills"


def test_registry_assets_reference_agent_skill_packages() -> None:
    registry = REGISTRY_ASSETS.read_text(encoding="utf-8")
    for skill_dir in sorted(path for path in AGENT_SKILLS_DIR.iterdir() if path.is_dir()):
        frontmatter, _ = _read_skill(skill_dir / "SKILL.md")
        name = frontmatter["name"]
        metadata = frontmatter["metadata"]

        assert f"id: {metadata['forge.asset_id']}" in registry
        assert f"name: {name}" in registry
        assert "format: agentskills.io/v1" in registry
        assert f"path: agent-skills/{name}" in registry
        assert f"runtime: {metadata['forge.runtime']}" in registry
        assert "eval_scores: {quality: 1.0, safety: 1.0, cost: 1.0, latency: 1.0}" in registry


def _read_skill(path: Path) -> tuple[dict[str, object], str]:
    text = path.read_text(encoding="utf-8")
    assert text.startswith("---\n")
    _, raw_frontmatter, body = text.split("---\n", 2)
    return _parse_frontmatter(raw_frontmatter), body


def _parse_frontmatter(raw: str) -> dict[str, object]:
    data: dict[str, object] = {}
    current_map: dict[str, str] | None = None
    for line in raw.splitlines():
        if not line.strip():
            continue
        if line.startswith("  "):
            assert current_map is not None
            key, value = line.strip().split(": ", 1)
            current_map[key] = value
            continue

        key, _, value = line.partition(": ")
        if value:
            data[key] = value
            current_map = None
        else:
            current_map = {}
            data[key.rstrip(":")] = current_map
    return data
