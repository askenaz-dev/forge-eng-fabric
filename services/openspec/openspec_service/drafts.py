"""Draft store for the Alfred wizard's progressive OpenSpec flow.

Drafts share the `openspec_id` namespace with committed OpenSpecs (per design
D2 of platform-gaps-closure) so that the commit transition is a no-op rename
plus validation. The store keeps drafts in memory by default and persists to a
per-Workspace JSON file when given a root path; production wiring will swap in
Postgres without changing the interface.
"""

from __future__ import annotations

import json
import threading
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from pathlib import Path

from openspec_service.models import (
    AuditInfo,
    AutonomyPolicy,
    CompletenessReport,
    CompletenessSection,
    IntentDraft,
    OpenSpecCreate,
    OpenSpecDocument,
    Requirements,
)


DRAFT_TTL = timedelta(days=14)
MAX_DIALOGUE_TURNS = 12


@dataclass
class DraftStore:
    """In-memory draft store with optional filesystem persistence."""

    root: Path | None = None
    drafts: dict[str, IntentDraft] = field(default_factory=dict)
    _lock: threading.RLock = field(default_factory=threading.RLock)

    def __post_init__(self) -> None:
        if self.root is not None:
            self.root.mkdir(parents=True, exist_ok=True)
            self._load_from_disk()

    def _load_from_disk(self) -> None:
        assert self.root is not None
        for path in self.root.glob("*.json"):
            try:
                data = json.loads(path.read_text(encoding="utf-8"))
                draft = IntentDraft.model_validate(data)
                self.drafts[draft.draft_id] = draft
            except (ValueError, json.JSONDecodeError):
                continue

    def _persist(self, draft: IntentDraft) -> None:
        if self.root is None:
            return
        path = self.root / f"{draft.draft_id}.json"
        path.write_text(draft.model_dump_json(indent=2) + "\n", encoding="utf-8")

    def _delete_disk(self, draft_id: str) -> None:
        if self.root is None:
            return
        path = self.root / f"{draft_id}.json"
        if path.exists():
            path.unlink()

    def create(
        self,
        *,
        workspace_id: uuid.UUID,
        created_by: str,
        title: str = "",
        business_intent: str = "",
        correlation_id: str | None = None,
    ) -> IntentDraft:
        draft = IntentDraft(
            draft_id=str(uuid.uuid4()),
            workspace_id=workspace_id,
            title=title,
            business_intent=business_intent,
            created_by=created_by,
            correlation_id=correlation_id,
        )
        with self._lock:
            self.drafts[draft.draft_id] = draft
            self._persist(draft)
        return draft

    def get(self, draft_id: str) -> IntentDraft | None:
        with self._lock:
            return self.drafts.get(draft_id)

    def update(self, draft: IntentDraft) -> IntentDraft:
        draft.updated_at = datetime.utcnow()
        draft.last_active_at = datetime.utcnow()
        with self._lock:
            self.drafts[draft.draft_id] = draft
            self._persist(draft)
        return draft

    def append_turn(
        self,
        draft_id: str,
        *,
        question: str | None,
        answer: str,
        field_updates: dict | None,
        actor: str,
    ) -> IntentDraft | None:
        with self._lock:
            draft = self.drafts.get(draft_id)
            if not draft:
                return None
            draft.turn_count += 1
            draft.dialogue_history.append(
                {
                    "turn": draft.turn_count,
                    "question": question,
                    "answer": answer,
                    "actor": actor,
                    "ts": datetime.utcnow().isoformat() + "Z",
                }
            )
            if field_updates:
                self._apply_field_updates(draft, field_updates)
            return self.update(draft)

    def _apply_field_updates(self, draft: IntentDraft, updates: dict) -> None:
        for key, value in updates.items():
            if key == "title":
                draft.title = str(value)
            elif key == "business_intent":
                draft.business_intent = str(value)
            elif key == "problem_statement":
                draft.problem_statement = str(value)
            elif key == "stakeholders" and isinstance(value, list):
                draft.stakeholders = [str(v) for v in value]
            elif key == "success_metrics" and isinstance(value, list):
                draft.success_metrics = [str(v) for v in value]
            elif key == "requirements_functional" and isinstance(value, list):
                draft.requirements.functional = [str(v) for v in value]
            elif key == "requirements_non_functional" and isinstance(value, list):
                draft.requirements.non_functional = [str(v) for v in value]
            elif key == "constraints" and isinstance(value, list):
                draft.constraints = [str(v) for v in value]
            elif key == "autonomy_policy" and isinstance(value, dict):
                draft.autonomy_policy = AutonomyPolicy.model_validate(value)

    def list(self, *, workspace_id: uuid.UUID | None = None) -> list[IntentDraft]:
        with self._lock:
            drafts = list(self.drafts.values())
        if workspace_id is not None:
            drafts = [d for d in drafts if d.workspace_id == workspace_id]
        return sorted(drafts, key=lambda d: d.last_active_at, reverse=True)

    def delete(self, draft_id: str) -> bool:
        with self._lock:
            if draft_id not in self.drafts:
                return False
            del self.drafts[draft_id]
            self._delete_disk(draft_id)
        return True

    def expire_inactive(self, *, now: datetime | None = None) -> list[IntentDraft]:
        """Mark drafts inactive for >= DRAFT_TTL as `abandoned`.

        Returns the list of drafts whose status flipped, so the caller can emit
        `intent.draft.abandoned.v1` audit events for each.
        """
        cutoff = (now or datetime.utcnow()) - DRAFT_TTL
        flipped: list[IntentDraft] = []
        with self._lock:
            for draft in list(self.drafts.values()):
                if draft.status != "drafting":
                    continue
                if draft.last_active_at < cutoff:
                    draft.status = "abandoned"
                    draft.updated_at = datetime.utcnow()
                    self._persist(draft)
                    flipped.append(draft)
        return flipped


def compute_completeness(draft: IntentDraft) -> CompletenessReport:
    """Compute the section/field completeness map the wizard renders.

    The map is also used to decide what question to ask next: the first section
    with `partial`/`empty` status drives the next-question prompt.
    """

    def status_for(value) -> str:
        if value is None:
            return "empty"
        if isinstance(value, str):
            return "complete" if value.strip() else "empty"
        if isinstance(value, list):
            return "complete" if len(value) > 0 else "empty"
        if isinstance(value, dict):
            return "complete" if value else "empty"
        return "complete"

    def aggregate(fields: dict[str, str]) -> str:
        statuses = list(fields.values())
        if not statuses:
            return "empty"
        if all(s == "complete" for s in statuses):
            return "complete"
        if any(s == "complete" for s in statuses):
            return "partial"
        return "empty"

    intent_fields = {
        "title": status_for(draft.title),
        "business_intent": status_for(draft.business_intent),
        "problem_statement": status_for(draft.problem_statement),
    }
    stakeholder_fields = {
        "stakeholders": status_for(draft.stakeholders),
        "success_metrics": status_for(draft.success_metrics),
    }
    req_fields = {
        "functional": status_for(draft.requirements.functional),
        "non_functional": status_for(draft.requirements.non_functional),
        "constraints": status_for(draft.constraints),
    }
    autonomy_fields = {
        "default_mode": status_for(draft.autonomy_policy.default_mode),
        "approvals_required": status_for(draft.autonomy_policy.approvals_required),
    }

    sections = [
        CompletenessSection(name="intent", status=aggregate(intent_fields), fields=intent_fields),
        CompletenessSection(name="stakeholders", status=aggregate(stakeholder_fields), fields=stakeholder_fields),
        CompletenessSection(name="requirements", status=aggregate(req_fields), fields=req_fields),
        CompletenessSection(name="autonomy", status=aggregate(autonomy_fields), fields=autonomy_fields),
    ]
    overall = aggregate({s.name: s.status for s in sections})
    return CompletenessReport(
        openspec_id=draft.openspec_id or draft.draft_id,
        overall=overall,
        sections=sections,
    )


def to_create_request(draft: IntentDraft) -> OpenSpecCreate:
    """Translate a draft into the existing OpenSpecCreate request shape so the
    commit step reuses the validated `create()` path."""
    return OpenSpecCreate(
        workspace_id=draft.workspace_id,
        title=draft.title or "(untitled intent)",
        business_intent=draft.business_intent,
        problem_statement=draft.problem_statement,
        stakeholders=draft.stakeholders,
        success_metrics=draft.success_metrics,
        requirements=draft.requirements,
        constraints=draft.constraints,
        autonomy_policy=draft.autonomy_policy,
        created_by=draft.created_by,
        openspec_id=draft.openspec_id,
        source="human",
    )


def can_commit(draft: IntentDraft) -> tuple[bool, str | None]:
    """Return (ok, reason) — drafts must have title + business intent + at least
    one functional requirement to commit, matching `OpenSpecCreate.validate_minimum_model`.
    """
    if not draft.business_intent.strip():
        return False, "business_intent is required"
    if not draft.requirements.functional:
        return False, "at least one functional requirement is required"
    if draft.turn_count > MAX_DIALOGUE_TURNS:
        return False, f"draft exceeded {MAX_DIALOGUE_TURNS} turns"
    return True, None
