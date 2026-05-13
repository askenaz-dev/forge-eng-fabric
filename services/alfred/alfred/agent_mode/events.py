"""CloudEvents emission helpers for agent-mode session lifecycle.

Follows the same envelope shape as `services/postmortem/postmortem/events.py`.
The sink is pluggable — by default events go to the log; production wiring
publishes to NATS/Kafka through the existing audit ingestion path.
"""

from __future__ import annotations

import json
import uuid
from abc import ABC, abstractmethod
from datetime import datetime, timezone
from typing import Any

from alfred.logging import get_logger

log = get_logger(__name__)

SOURCE = "forge://service/alfred"

# Registry of supported agent-mode event types (used for validation and docs).
EVENT_TYPES = (
    "alfred.agent_mode.session_started.v1",
    "alfred.agent_mode.step_started.v1",
    "alfred.agent_mode.step_completed.v1",
    "alfred.agent_mode.plan_revised.v1",
    "alfred.agent_mode.paused_for_approval.v1",
    "alfred.agent_mode.paused_for_budget.v1",
    "alfred.agent_mode.resumed.v1",
    "alfred.agent_mode.completed.v1",
    "alfred.agent_mode.aborted.v1",
    "alfred.agent_mode.failed.v1",
)


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def new_event(
    *,
    event_type: str,
    subject: str,
    workspace_id: str | None,
    tenant_id: str | None,
    data: dict[str, Any],
) -> dict[str, Any]:
    return {
        "specversion": "1.0",
        "id": str(uuid.uuid4()),
        "source": SOURCE,
        "type": event_type,
        "subject": subject,
        "time": utcnow().isoformat(),
        "datacontenttype": "application/json",
        "forgetenantid": tenant_id or "",
        "forgeworkspaceid": workspace_id or "",
        "data": data,
    }


class Sink(ABC):
    @abstractmethod
    async def emit(self, event: dict[str, Any]) -> None: ...


class MemorySink(Sink):
    def __init__(self) -> None:
        self.events: list[dict[str, Any]] = []

    async def emit(self, event: dict[str, Any]) -> None:
        self.events.append(event)


class LogSink(Sink):
    async def emit(self, event: dict[str, Any]) -> None:
        log.info("agent_mode_event", event=json.dumps(event, default=str))


class EventEmitter:
    """Closure-friendly emitter — pass `.emit` to the executor."""

    def __init__(self, sink: Sink) -> None:
        self.sink = sink

    async def emit(self, event_type: str, payload: dict[str, Any]) -> None:
        if event_type not in EVENT_TYPES:
            log.warning("unknown_agent_mode_event_type", type=event_type)
        subject = f"agent-mode:{payload.get('session_id', 'unknown')}"
        await self.sink.emit(
            new_event(
                event_type=event_type,
                subject=subject,
                workspace_id=str(payload.get("workspace_id") or ""),
                tenant_id=None,
                data=payload,
            )
        )
