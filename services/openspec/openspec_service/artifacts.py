from __future__ import annotations

import re
from pathlib import Path

from openspec_service.models import OpenSpecArtifactRef, OpenSpecDocument


class OpenSpecArtifactWriter:
    """Projects Forge structured specifications into real OpenSpec change files."""

    def __init__(self, root: Path) -> None:
        self.root = root

    def write(self, document: OpenSpecDocument) -> OpenSpecArtifactRef:
        change_id = _slug(document.openspec_id)
        change_dir = self.root / "changes" / change_id
        spec_dir = change_dir / "specs" / change_id
        spec_dir.mkdir(parents=True, exist_ok=True)

        files = {
            ".openspec.yaml": _metadata(document),
            "README.md": _readme(document),
            f"specs/{change_id}/spec.md": _spec_delta(document),
        }
        for relative, content in files.items():
            path = change_dir / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content.rstrip() + "\n", encoding="utf-8")

        return OpenSpecArtifactRef(
            change_id=change_id,
            root=str(change_dir.resolve()),
            files=list(files.keys()),
        )


def _metadata(document: OpenSpecDocument) -> str:
    return "\n".join(
        [
            "schema: spec-driven",
            f"created: {document.audit.created_at.date().isoformat()}",
            f"updated: {document.audit.updated_at.date().isoformat() if document.audit.updated_at else document.audit.created_at.date().isoformat()}",
            "source: forge-specification-service",
            f"forge_openspec_id: {document.openspec_id}",
            f"forge_workspace_id: {document.workspace_id}",
        ]
    )


def _readme(document: OpenSpecDocument) -> str:
    lines = [
        f"# {document.title}",
        "",
        f"Source specification: `{document.openspec_id}`",
        f"Workspace: `{document.workspace_id}`",
        f"Version: `{document.version}`",
        "",
        "## Business Intent",
        document.business_intent or "_Not provided_",
        "",
        "## Problem Statement",
        document.problem_statement or "_Not provided_",
        "",
        "## Functional Requirements",
        *_bullets(document.requirements.functional),
        "",
        "## Non-Functional Requirements",
        *_bullets(document.requirements.non_functional),
        "",
        "## Constraints",
        *_bullets(document.constraints),
        "",
        "## Linked Artifacts",
        *_bullets([f"{link.kind}: {link.ref}" for link in document.linked_artifacts]),
        "",
        "## Audit",
        f"- Created by `{document.audit.created_by}` at `{document.audit.created_at.isoformat()}`",
    ]
    if document.audit.updated_by and document.audit.updated_at:
        lines.append(f"- Updated by `{document.audit.updated_by}` at `{document.audit.updated_at.isoformat()}`")
    return "\n".join(lines)


def _spec_delta(document: OpenSpecDocument) -> str:
    lines = [
        "## ADDED Requirements",
        "",
        f"### Requirement: {document.title}",
        "",
        f"The delivered capability SHALL satisfy the structured specification `{document.openspec_id}`.",
        "",
        "#### Scenario: Business intent is met",
        "",
        "- **WHEN** the initiative is delivered",
        f"- **THEN** {document.business_intent}",
    ]

    for index, requirement in enumerate(document.requirements.functional, start=1):
        lines.extend(
            [
                "",
                f"### Requirement: Functional requirement {index}",
                "",
                f"The delivered capability SHALL {_sentence(requirement)}",
                "",
                f"#### Scenario: Functional requirement {index} is verified",
                "",
                "- **WHEN** acceptance evidence is collected for this initiative",
                f"- **THEN** {requirement}",
            ]
        )

    for index, requirement in enumerate(document.requirements.non_functional, start=1):
        lines.extend(
            [
                "",
                f"### Requirement: Non-functional requirement {index}",
                "",
                f"The delivered capability SHALL meet `{requirement}`.",
                "",
                f"#### Scenario: Non-functional requirement {index} is verified",
                "",
                "- **WHEN** operational or test evidence is collected",
                f"- **THEN** {requirement}",
            ]
        )

    return "\n".join(lines)


def _bullets(values: list[str]) -> list[str]:
    if not values:
        return ["_None recorded_"]
    return [f"- {value}" for value in values]


def _sentence(value: str) -> str:
    text = value.strip()
    if not text:
        return "satisfy the recorded requirement."
    text = text[0].lower() + text[1:] if text[0].isupper() else text
    return text if text.endswith(".") else f"{text}."


def _slug(value: str) -> str:
    slug = re.sub(r"[^a-z0-9-]+", "-", value.lower().replace("_", "-")).strip("-")
    return slug[:80] or "specification"
