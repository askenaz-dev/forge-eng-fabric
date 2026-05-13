"""Postgres-backed persistence for agent-mode sessions and steps.

Extends the existing Alfred Store with `alfred_agent_session` and
`alfred_agent_step` operations. The in-memory variant mirrors the same surface
for tests.
"""

from __future__ import annotations

import json
import uuid
from datetime import datetime
from typing import Any

from alfred.agent_mode.models import AgentModeSession, AgentModeStep, PlanRevision, SessionStatus
from alfred.store import InMemoryStore, Store


class AgentModeStore:
    """Thin façade over the underlying Store for the agent-mode tables."""

    def __init__(self, store: Store) -> None:
        self._store = store
        self._in_memory: bool = isinstance(store, InMemoryStore)
        if self._in_memory:
            self._sessions: dict[uuid.UUID, AgentModeSession] = {}
            self._steps: dict[uuid.UUID, list[AgentModeStep]] = {}
            self._revisions: dict[uuid.UUID, list[PlanRevision]] = {}

    # --- sessions ------------------------------------------------------

    async def create_session(self, s: AgentModeSession) -> AgentModeSession:
        if self._in_memory:
            self._sessions[s.id] = s
            self._steps[s.id] = []
            self._revisions[s.id] = []
            return s
        async with self._store.acquire() as c:
            await c.execute(
                """
                INSERT INTO alfred_agent_session (
                  id, workspace_id, openspec_id, correlation_id, originator_principal,
                  model_id, plan_revision, plan_json, frozen_autonomy_policy, status,
                  started_at, paused_at, resumed_at, completed_at, aborted_reason,
                  workflow_run_id
                ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10,$11,$12,$13,$14,$15,$16)
                """,
                s.id, s.workspace_id, s.openspec_id, s.correlation_id,
                s.originator_principal, s.model_id, s.plan_revision,
                json.dumps(s.plan_json), json.dumps(s.frozen_autonomy_policy),
                s.status, s.started_at, s.paused_at, s.resumed_at,
                s.completed_at, s.aborted_reason, s.workflow_run_id,
            )
        return s

    async def get_session(self, session_id: uuid.UUID) -> AgentModeSession | None:
        if self._in_memory:
            return self._sessions.get(session_id)
        async with self._store.acquire() as c:
            row = await c.fetchrow(
                "SELECT * FROM alfred_agent_session WHERE id = $1", session_id
            )
        if not row:
            return None
        return AgentModeSession(
            id=row["id"],
            workspace_id=row["workspace_id"],
            openspec_id=row["openspec_id"],
            correlation_id=row["correlation_id"],
            originator_principal=row["originator_principal"],
            model_id=row["model_id"],
            plan_revision=row["plan_revision"],
            plan_json=json.loads(row["plan_json"]) if row["plan_json"] else {},
            frozen_autonomy_policy=(
                json.loads(row["frozen_autonomy_policy"]) if row["frozen_autonomy_policy"] else {}
            ),
            status=row["status"],
            started_at=row["started_at"],
            paused_at=row["paused_at"],
            resumed_at=row["resumed_at"],
            completed_at=row["completed_at"],
            aborted_reason=row["aborted_reason"],
            workflow_run_id=row["workflow_run_id"],
        )

    async def list_sessions(
        self, *, workspace_id: uuid.UUID | None = None, limit: int = 100
    ) -> list[AgentModeSession]:
        if self._in_memory:
            out = list(self._sessions.values())
            if workspace_id:
                out = [s for s in out if s.workspace_id == workspace_id]
            out.sort(key=lambda s: s.started_at, reverse=True)
            return out[:limit]
        clauses: list[str] = []
        params: list[Any] = []
        if workspace_id:
            params.append(workspace_id)
            clauses.append(f"workspace_id = ${len(params)}")
        where = ("WHERE " + " AND ".join(clauses)) if clauses else ""
        params.append(limit)
        sql = (
            f"SELECT * FROM alfred_agent_session {where} "
            f"ORDER BY started_at DESC LIMIT ${len(params)}"
        )
        async with self._store.acquire() as c:
            rows = await c.fetch(sql, *params)
        return [
            AgentModeSession(
                id=r["id"], workspace_id=r["workspace_id"], openspec_id=r["openspec_id"],
                correlation_id=r["correlation_id"], originator_principal=r["originator_principal"],
                model_id=r["model_id"], plan_revision=r["plan_revision"],
                plan_json=json.loads(r["plan_json"]) if r["plan_json"] else {},
                frozen_autonomy_policy=(
                    json.loads(r["frozen_autonomy_policy"]) if r["frozen_autonomy_policy"] else {}
                ),
                status=r["status"], started_at=r["started_at"],
                paused_at=r["paused_at"], resumed_at=r["resumed_at"],
                completed_at=r["completed_at"], aborted_reason=r["aborted_reason"],
                workflow_run_id=r["workflow_run_id"],
            )
            for r in rows
        ]

    async def update_session(
        self,
        session_id: uuid.UUID,
        *,
        status: SessionStatus | None = None,
        plan_revision: int | None = None,
        plan_json: dict[str, Any] | None = None,
        paused_at: datetime | None = None,
        resumed_at: datetime | None = None,
        completed_at: datetime | None = None,
        aborted_reason: str | None = None,
        workflow_run_id: str | None = None,
    ) -> AgentModeSession | None:
        if self._in_memory:
            existing = self._sessions.get(session_id)
            if not existing:
                return None
            updates: dict[str, Any] = {}
            if status is not None:
                updates["status"] = status
            if plan_revision is not None:
                updates["plan_revision"] = plan_revision
            if plan_json is not None:
                updates["plan_json"] = plan_json
            if paused_at is not None:
                updates["paused_at"] = paused_at
            if resumed_at is not None:
                updates["resumed_at"] = resumed_at
            if completed_at is not None:
                updates["completed_at"] = completed_at
            if aborted_reason is not None:
                updates["aborted_reason"] = aborted_reason
            if workflow_run_id is not None:
                updates["workflow_run_id"] = workflow_run_id
            updated = existing.model_copy(update=updates)
            self._sessions[session_id] = updated
            return updated
        sets: list[str] = []
        params: list[Any] = []

        def _set(col: str, val: Any, *, jsonb: bool = False) -> None:
            params.append(val)
            sets.append(f"{col} = ${len(params)}{'::jsonb' if jsonb else ''}")

        if status is not None:
            _set("status", status)
        if plan_revision is not None:
            _set("plan_revision", plan_revision)
        if plan_json is not None:
            _set("plan_json", json.dumps(plan_json), jsonb=True)
        if paused_at is not None:
            _set("paused_at", paused_at)
        if resumed_at is not None:
            _set("resumed_at", resumed_at)
        if completed_at is not None:
            _set("completed_at", completed_at)
        if aborted_reason is not None:
            _set("aborted_reason", aborted_reason)
        if workflow_run_id is not None:
            _set("workflow_run_id", workflow_run_id)
        if not sets:
            return await self.get_session(session_id)
        params.append(session_id)
        sql = (
            f"UPDATE alfred_agent_session SET {', '.join(sets)} "
            f"WHERE id = ${len(params)}"
        )
        async with self._store.acquire() as c:
            await c.execute(sql, *params)
        return await self.get_session(session_id)

    # --- steps ---------------------------------------------------------

    async def upsert_step(self, step: AgentModeStep) -> AgentModeStep:
        if self._in_memory:
            steps = self._steps.setdefault(step.session_id, [])
            for i, existing in enumerate(steps):
                if existing.id == step.id:
                    steps[i] = step
                    return step
            steps.append(step)
            return step
        async with self._store.acquire() as c:
            await c.execute(
                """
                INSERT INTO alfred_agent_step (
                  id, session_id, idx, kind, tool_id, workflow_id, agent_id,
                  criticality, decision_id, status, started_at, completed_at, outcome
                ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)
                ON CONFLICT (id) DO UPDATE SET
                  status = EXCLUDED.status,
                  decision_id = EXCLUDED.decision_id,
                  started_at = EXCLUDED.started_at,
                  completed_at = EXCLUDED.completed_at,
                  outcome = EXCLUDED.outcome
                """,
                step.id, step.session_id, step.idx, step.kind, step.tool_id,
                step.workflow_id, step.agent_id, step.criticality, step.decision_id,
                step.status, step.started_at, step.completed_at,
                json.dumps(step.outcome) if step.outcome else None,
            )
        return step

    async def list_steps(self, session_id: uuid.UUID) -> list[AgentModeStep]:
        if self._in_memory:
            return sorted(self._steps.get(session_id, []), key=lambda s: s.idx)
        async with self._store.acquire() as c:
            rows = await c.fetch(
                "SELECT * FROM alfred_agent_step WHERE session_id=$1 ORDER BY idx ASC",
                session_id,
            )
        return [
            AgentModeStep(
                id=r["id"], session_id=r["session_id"], idx=r["idx"],
                kind=r["kind"], tool_id=r["tool_id"], workflow_id=r["workflow_id"],
                agent_id=r["agent_id"], criticality=r["criticality"],
                decision_id=r["decision_id"], status=r["status"],
                started_at=r["started_at"], completed_at=r["completed_at"],
                outcome=json.loads(r["outcome"]) if r["outcome"] else None,
            )
            for r in rows
        ]

    async def append_revision(self, session_id: uuid.UUID, rev: PlanRevision) -> None:
        if self._in_memory:
            self._revisions.setdefault(session_id, []).append(rev)

    async def list_revisions(self, session_id: uuid.UUID) -> list[PlanRevision]:
        if self._in_memory:
            return list(self._revisions.get(session_id, []))
        return []
