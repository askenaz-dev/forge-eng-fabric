"""Postgres-backed persistence for sessions, messages, and decisions."""

from __future__ import annotations

import json
import uuid
from contextlib import asynccontextmanager
from datetime import datetime
from typing import Any, AsyncIterator

import asyncpg

from alfred.models import DecisionRecord, Session


class Store:
    def __init__(self, dsn: str) -> None:
        self._dsn = dsn
        self._pool: asyncpg.Pool | None = None

    async def connect(self) -> None:
        self._pool = await asyncpg.create_pool(dsn=self._dsn, min_size=1, max_size=10)

    async def close(self) -> None:
        if self._pool:
            await self._pool.close()

    @asynccontextmanager
    async def acquire(self) -> AsyncIterator[asyncpg.Connection]:
        assert self._pool, "store not connected"
        async with self._pool.acquire() as conn:
            yield conn

    async def ping(self) -> None:
        async with self.acquire() as c:
            await c.execute("SELECT 1")

    # --- sessions -------------------------------------------------------

    async def create_session(self, s: Session) -> Session:
        async with self.acquire() as c:
            await c.execute(
                """
                INSERT INTO alfred_session (id, workspace_id, actor, started_at,
                                            last_activity_at, status, correlation_id, metadata)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
                """,
                s.id,
                s.workspace_id,
                s.actor,
                s.started_at,
                s.last_activity_at,
                s.status,
                s.correlation_id,
                json.dumps(s.metadata),
            )
        return s

    async def get_session(self, session_id: uuid.UUID) -> Session | None:
        async with self.acquire() as c:
            row = await c.fetchrow(
                "SELECT * FROM alfred_session WHERE id = $1", session_id
            )
        if not row:
            return None
        return Session(
            id=row["id"],
            workspace_id=row["workspace_id"],
            actor=row["actor"],
            started_at=row["started_at"],
            last_activity_at=row["last_activity_at"],
            status=row["status"],
            correlation_id=row["correlation_id"],
            metadata=json.loads(row["metadata"]) if row["metadata"] else {},
        )

    async def append_message(
        self,
        *,
        session_id: uuid.UUID,
        role: str,
        content: str,
        tool_call_id: str | None,
    ) -> uuid.UUID:
        msg_id = uuid.uuid4()
        async with self.acquire() as c:
            await c.execute(
                """
                INSERT INTO alfred_message (id, session_id, role, content, tool_call_id, created_at)
                VALUES ($1, $2, $3, $4, $5, $6)
                """,
                msg_id,
                session_id,
                role,
                content,
                tool_call_id,
                datetime.utcnow(),
            )
            await c.execute(
                "UPDATE alfred_session SET last_activity_at=$1 WHERE id=$2",
                datetime.utcnow(),
                session_id,
            )
        return msg_id

    async def list_messages(self, session_id: uuid.UUID) -> list[dict[str, Any]]:
        async with self.acquire() as c:
            rows = await c.fetch(
                """
                SELECT id, role, content, tool_call_id, created_at
                FROM alfred_message WHERE session_id=$1 ORDER BY created_at ASC
                """,
                session_id,
            )
        return [dict(r) for r in rows]

    # --- decisions ------------------------------------------------------

    async def append_decision(self, d: DecisionRecord) -> None:
        async with self.acquire() as c:
            await c.execute(
                """
                INSERT INTO alfred_decision
                  (id, session_id, workspace_id, actor, correlation_id, intent,
                   retrieved_refs, policy_evaluated, tool_kind, tool_id, params_redacted,
                   outcome, outcome_detail, occurred_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10,$11::jsonb,$12,$13::jsonb,$14)
                """,
                d.id,
                d.session_id,
                d.workspace_id,
                d.actor,
                d.correlation_id,
                d.intent,
                json.dumps(d.retrieved_refs),
                json.dumps(d.policy_evaluated) if d.policy_evaluated else None,
                d.tool_kind,
                d.tool_id,
                json.dumps(d.params_redacted),
                d.outcome,
                json.dumps(d.outcome_detail) if d.outcome_detail else None,
                d.occurred_at,
            )

    async def list_decisions(
        self,
        *,
        workspace_id: uuid.UUID | None = None,
        session_id: uuid.UUID | None = None,
        correlation_id: str | None = None,
        limit: int = 100,
    ) -> list[DecisionRecord]:
        clauses: list[str] = []
        params: list[Any] = []
        if workspace_id:
            params.append(workspace_id)
            clauses.append(f"workspace_id = ${len(params)}")
        if session_id:
            params.append(session_id)
            clauses.append(f"session_id = ${len(params)}")
        if correlation_id:
            params.append(correlation_id)
            clauses.append(f"correlation_id = ${len(params)}")
        where = ("WHERE " + " AND ".join(clauses)) if clauses else ""
        params.append(limit)
        sql = f"SELECT * FROM alfred_decision {where} ORDER BY occurred_at DESC LIMIT ${len(params)}"
        async with self.acquire() as c:
            rows = await c.fetch(sql, *params)
        out: list[DecisionRecord] = []
        for r in rows:
            out.append(
                DecisionRecord(
                    id=r["id"],
                    session_id=r["session_id"],
                    workspace_id=r["workspace_id"],
                    actor=r["actor"],
                    correlation_id=r["correlation_id"],
                    intent=r["intent"],
                    retrieved_refs=json.loads(r["retrieved_refs"]) if r["retrieved_refs"] else [],
                    policy_evaluated=json.loads(r["policy_evaluated"]) if r["policy_evaluated"] else None,
                    tool_kind=r["tool_kind"],
                    tool_id=r["tool_id"],
                    params_redacted=json.loads(r["params_redacted"]) if r["params_redacted"] else {},
                    outcome=r["outcome"],
                    outcome_detail=json.loads(r["outcome_detail"]) if r["outcome_detail"] else None,
                    occurred_at=r["occurred_at"],
                )
            )
        return out


class InMemoryStore(Store):
    """No-DB fallback used in tests and dev when Postgres is unavailable."""

    def __init__(self) -> None:
        super().__init__(dsn="")
        self._sessions: dict[uuid.UUID, Session] = {}
        self._messages: dict[uuid.UUID, list[dict[str, Any]]] = {}
        self._decisions: list[DecisionRecord] = []

    async def connect(self) -> None:
        return None

    async def close(self) -> None:
        return None

    async def ping(self) -> None:
        return None

    async def create_session(self, s: Session) -> Session:
        self._sessions[s.id] = s
        self._messages[s.id] = []
        return s

    async def get_session(self, session_id: uuid.UUID) -> Session | None:
        return self._sessions.get(session_id)

    async def append_message(self, *, session_id, role, content, tool_call_id):
        msg_id = uuid.uuid4()
        self._messages.setdefault(session_id, []).append(
            {
                "id": msg_id,
                "role": role,
                "content": content,
                "tool_call_id": tool_call_id,
                "created_at": datetime.utcnow(),
            }
        )
        if session_id in self._sessions:
            self._sessions[session_id] = self._sessions[session_id].model_copy(
                update={"last_activity_at": datetime.utcnow()}
            )
        return msg_id

    async def list_messages(self, session_id):
        return list(self._messages.get(session_id, []))

    async def append_decision(self, d: DecisionRecord) -> None:
        self._decisions.append(d)

    async def list_decisions(
        self,
        *,
        workspace_id=None,
        session_id=None,
        correlation_id=None,
        limit=100,
    ):
        out = list(self._decisions)
        if workspace_id:
            out = [d for d in out if d.workspace_id == workspace_id]
        if session_id:
            out = [d for d in out if d.session_id == session_id]
        if correlation_id:
            out = [d for d in out if d.correlation_id == correlation_id]
        out.sort(key=lambda d: d.occurred_at, reverse=True)
        return out[:limit]
