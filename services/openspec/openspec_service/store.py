from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

from openspec_service.models import (
    AuditInfo,
    DecisionLogEntry,
    LinkedArtifact,
    OpenSpecCreate,
    OpenSpecDocument,
    OpenSpecPatch,
)


@dataclass
class InMemoryIndex:
    rows: dict[str, OpenSpecDocument] = field(default_factory=dict)

    def upsert(self, document: OpenSpecDocument) -> None:
        self.rows[document.openspec_id] = document

    def remove(self, openspec_id: str) -> None:
        self.rows.pop(openspec_id, None)


@dataclass
class FilesystemOpenSpecStore:
    root: Path
    index: InMemoryIndex = field(default_factory=InMemoryIndex)

    def __post_init__(self) -> None:
        self.root.mkdir(parents=True, exist_ok=True)
        (self.root / ".versions").mkdir(parents=True, exist_ok=True)
        self.sync_from_filesystem()

    def list(self, workspace_id: str | None = None) -> list[OpenSpecDocument]:
        docs = list(self.index.rows.values())
        if workspace_id:
            docs = [doc for doc in docs if str(doc.workspace_id) == workspace_id]
        return sorted(docs, key=lambda doc: (str(doc.workspace_id), doc.openspec_id))

    def get(self, openspec_id: str) -> OpenSpecDocument | None:
        return self.index.rows.get(openspec_id)

    def list_versions(self, openspec_id: str) -> list[int]:
        version_dir = self.root / ".versions" / openspec_id
        if not version_dir.exists():
            return []
        versions: list[int] = []
        for path in version_dir.glob("v*.json"):
            try:
                versions.append(int(path.stem.removeprefix("v")))
            except ValueError:
                continue
        return sorted(versions)

    def get_version(self, openspec_id: str, version: int) -> OpenSpecDocument | None:
        path = self.root / ".versions" / openspec_id / f"v{version}.json"
        if not path.exists():
            return None
        return OpenSpecDocument.model_validate(json.loads(path.read_text(encoding="utf-8")))

    def create(self, request: OpenSpecCreate) -> OpenSpecDocument:
        openspec_id = request.openspec_id or _slug(request.title)
        if self.get(openspec_id):
            raise ValueError(f"openspec {openspec_id!r} already exists")
        # Autonomous-loop changes start as `pending`; humans default to
        # `approved` (the existing implicit behaviour).
        review_status = "pending" if request.source == "autonomous-loop" else "approved"
        document = OpenSpecDocument(
            openspec_id=openspec_id,
            workspace_id=request.workspace_id,
            title=request.title,
            business_intent=request.business_intent,
            problem_statement=request.problem_statement,
            stakeholders=request.stakeholders,
            success_metrics=request.success_metrics,
            requirements=request.requirements,
            constraints=request.constraints,
            autonomy_policy=request.autonomy_policy,
            linked_artifacts=request.linked_artifacts,
            audit=AuditInfo(created_by=request.created_by),
            source=request.source,
            review_status=review_status,
        )
        self._write(document)
        return document

    def review(
        self,
        openspec_id: str,
        *,
        approved: bool,
        reviewer: str,
        comment: str | None = None,
    ) -> OpenSpecDocument | None:
        document = self.get(openspec_id)
        if not document:
            return None
        if document.source != "autonomous-loop":
            raise ValueError("review is only valid for autonomous-loop documents")
        if document.review_status != "pending":
            raise ValueError(f"document already reviewed: {document.review_status}")
        updated = document.model_copy(deep=True)
        updated.review_status = "approved" if approved else "rejected"
        updated.reviewed_by = reviewer
        updated.review_comment = comment
        updated.version += 1
        updated.audit.updated_by = reviewer
        updated.audit.updated_at = _utcnow()
        self._write(updated)
        return updated

    def evolution_stats(self) -> dict[str, float | int]:
        total = pending = approved = rejected = 0
        for doc in self.index.rows.values():
            if doc.source != "autonomous-loop":
                continue
            total += 1
            if doc.review_status == "pending":
                pending += 1
            elif doc.review_status == "approved":
                approved += 1
            elif doc.review_status == "rejected":
                rejected += 1
        decided = approved + rejected
        ratio = (approved / decided) if decided else 0.0
        return {
            "total": total,
            "pending": pending,
            "approved": approved,
            "rejected": rejected,
            "acceptance_ratio": ratio,
        }

    def patch(self, openspec_id: str, request: OpenSpecPatch) -> OpenSpecDocument | None:
        document = self.get(openspec_id)
        if not document:
            return None
        patch = request.model_dump(exclude_unset=True)
        updated_by = patch.pop("updated_by")
        updated = document.model_copy(update=patch)
        updated.audit.updated_by = updated_by
        updated.audit.updated_at = _utcnow()
        updated.version = document.version + 1
        validated = OpenSpecDocument.model_validate(updated.model_dump())
        self._write(validated)
        return validated

    def append_decision(self, openspec_id: str, decision: DecisionLogEntry) -> OpenSpecDocument | None:
        document = self.get(openspec_id)
        if not document:
            return None
        updated = document.model_copy(deep=True)
        updated.decision_log.append(decision)
        updated.version += 1
        updated.audit.updated_by = decision.actor
        updated.audit.updated_at = _utcnow()
        self._write(updated)
        return updated

    def append_link(self, openspec_id: str, link: LinkedArtifact, actor: str) -> OpenSpecDocument | None:
        document = self.get(openspec_id)
        if not document:
            return None
        updated = document.model_copy(deep=True)
        updated.linked_artifacts.append(link)
        updated.version += 1
        updated.audit.updated_by = actor
        updated.audit.updated_at = _utcnow()
        self._write(updated)
        return updated

    def sync_from_filesystem(self) -> None:
        self.index.rows.clear()
        for path in self.root.glob("*/*.json"):
            document = OpenSpecDocument.model_validate(json.loads(path.read_text(encoding="utf-8")))
            self.index.upsert(document)

    def _write(self, document: OpenSpecDocument) -> None:
        ws_dir = self.root / str(document.workspace_id)
        ws_dir.mkdir(parents=True, exist_ok=True)
        data = document.model_dump_json(indent=2)
        (ws_dir / f"{document.openspec_id}.json").write_text(data + "\n", encoding="utf-8")
        version_dir = self.root / ".versions" / document.openspec_id
        version_dir.mkdir(parents=True, exist_ok=True)
        (version_dir / f"v{document.version}.json").write_text(data + "\n", encoding="utf-8")
        self.index.upsert(document)


def _slug(value: str) -> str:
    slug = "-".join(part for part in value.lower().replace("_", "-").split() if part)
    return slug[:80] or "openspec"


def _utcnow():
    from datetime import datetime

    return datetime.utcnow()
