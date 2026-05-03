from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path

from approvals.models import ApprovalCreate, ApprovalDecision, ApprovalRequest


@dataclass
class InMemoryApprovalStore:
    approvals: dict[str, ApprovalRequest] = field(default_factory=dict)

    def create(self, request: ApprovalCreate) -> ApprovalRequest:
        approval = ApprovalRequest.from_create(request)
        self.approvals[approval.id] = approval
        return approval

    def get(self, approval_id: str) -> ApprovalRequest | None:
        return self.approvals.get(approval_id)

    def list(
        self,
        *,
        approver: str | None = None,
        status: str | None = None,
        workspace_id: str | None = None,
    ) -> list[ApprovalRequest]:
        rows = list(self.approvals.values())
        if approver:
            rows = [row for row in rows if approver in row.required_approvers]
        if status:
            rows = [row for row in rows if row.status == status]
        if workspace_id:
            rows = [row for row in rows if str(row.workspace_id) == workspace_id]
        return sorted(rows, key=lambda row: row.requested_at, reverse=True)

    def decide(self, approval_id: str, decision: ApprovalDecision) -> ApprovalRequest | None:
        approval = self.approvals.get(approval_id)
        if not approval or approval.status != "pending":
            return approval
        updated = approval.model_copy(
            update={
                "status": decision.decision,
                "decided_by": decision.actor,
                "decided_at": datetime.utcnow(),
                "decision_comment": decision.comment,
            }
        )
        self.approvals[approval_id] = updated
        return updated

    def expire_due(self, now: datetime | None = None) -> list[ApprovalRequest]:
        now = now or datetime.utcnow()
        expired: list[ApprovalRequest] = []
        for approval in list(self.approvals.values()):
            if approval.status == "pending" and approval.expires_at <= now:
                updated = approval.model_copy(update={"status": "expired"})
                self.approvals[approval.id] = updated
                expired.append(updated)
        return expired


class FileApprovalStore(InMemoryApprovalStore):
    """Small durable store for local/bootstrap deployments.

    Postgres is represented by `db/migrations/approvals`; this file-backed store
    gives the FastAPI service restart durability without requiring DB wiring in
    the bootstrap slice.
    """

    def __init__(self, path: Path) -> None:
        super().__init__()
        self.path = path
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._load()

    def create(self, request: ApprovalCreate) -> ApprovalRequest:
        approval = super().create(request)
        self._persist()
        return approval

    def decide(self, approval_id: str, decision: ApprovalDecision) -> ApprovalRequest | None:
        approval = super().decide(approval_id, decision)
        self._persist()
        return approval

    def expire_due(self, now: datetime | None = None) -> list[ApprovalRequest]:
        expired = super().expire_due(now)
        if expired:
            self._persist()
        return expired

    def _load(self) -> None:
        if not self.path.exists():
            return
        data = json.loads(self.path.read_text(encoding="utf-8"))
        self.approvals = {
            item["id"]: ApprovalRequest.model_validate(item)
            for item in data.get("approvals", [])
        }

    def _persist(self) -> None:
        data = {"approvals": [approval.model_dump(mode="json") for approval in self.approvals.values()]}
        self.path.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")
