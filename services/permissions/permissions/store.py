from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path

from permissions.models import (
    AuditEntry,
    CheckRequest,
    CheckResponse,
    DelegatedPermission,
    GrantCreate,
    RevokeRequest,
)

CRITICALITY_RANK = {"low": 1, "medium": 2, "high": 3, "critical": 4}


@dataclass
class InMemoryPermissionStore:
    grants: dict[str, DelegatedPermission] = field(default_factory=dict)

    def create(self, request: GrantCreate) -> DelegatedPermission:
        grant = DelegatedPermission.from_create(request)
        self.grants[grant.id] = grant
        return grant

    def list(self, *, subject: str | None = None, scope_id: str | None = None, status: str | None = None) -> list[DelegatedPermission]:
        rows = list(self.grants.values())
        if subject:
            rows = [grant for grant in rows if grant.subject == subject]
        if scope_id:
            rows = [grant for grant in rows if grant.scope_id == scope_id]
        if status:
            rows = [grant for grant in rows if grant.status == status]
        return sorted(rows, key=lambda grant: grant.expires_at)

    def revoke(self, grant_id: str, request: RevokeRequest) -> DelegatedPermission | None:
        grant = self.grants.get(grant_id)
        if not grant:
            return None
        updated = grant.model_copy(deep=True)
        updated.status = "revoked"
        updated.audit_history.append(AuditEntry(actor=request.actor, action="revoked", rationale=request.rationale))
        self.grants[grant_id] = updated
        return updated

    def expire_due(self, now: datetime | None = None) -> list[DelegatedPermission]:
        now = now or datetime.utcnow()
        expired: list[DelegatedPermission] = []
        for grant in list(self.grants.values()):
            if grant.status == "active" and grant.expires_at <= now:
                updated = grant.model_copy(deep=True)
                updated.status = "expired"
                updated.audit_history.append(AuditEntry(actor="system", action="expired"))
                self.grants[grant.id] = updated
                expired.append(updated)
        return expired

    def check(self, request: CheckRequest) -> CheckResponse:
        self.expire_due()
        for grant in self.grants.values():
            if not _matches(grant, request):
                continue
            return CheckResponse(allowed=True, reason="active delegated permission", grant_id=grant.id)
        return CheckResponse(allowed=False, reason="no active delegated permission")


class FilePermissionStore(InMemoryPermissionStore):
    def __init__(self, path: Path) -> None:
        super().__init__()
        self.path = path
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._load()

    def create(self, request: GrantCreate) -> DelegatedPermission:
        grant = super().create(request)
        self._persist()
        return grant

    def revoke(self, grant_id: str, request: RevokeRequest) -> DelegatedPermission | None:
        grant = super().revoke(grant_id, request)
        self._persist()
        return grant

    def expire_due(self, now: datetime | None = None) -> list[DelegatedPermission]:
        expired = super().expire_due(now)
        if expired:
            self._persist()
        return expired

    def _load(self) -> None:
        if not self.path.exists():
            return
        data = json.loads(self.path.read_text(encoding="utf-8"))
        self.grants = {item["id"]: DelegatedPermission.model_validate(item) for item in data.get("grants", [])}

    def _persist(self) -> None:
        data = {"grants": [grant.model_dump(mode="json") for grant in self.grants.values()]}
        self.path.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def _matches(grant: DelegatedPermission, request: CheckRequest) -> bool:
    return (
        grant.status == "active"
        and grant.subject == request.subject
        and grant.action_class == request.action_class
        and grant.scope_kind == request.scope_kind
        and grant.scope_id == request.scope_id
        and CRITICALITY_RANK[request.criticality] <= CRITICALITY_RANK[grant.max_criticality]
    )
