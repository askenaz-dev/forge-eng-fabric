"""CloudEvent emission helpers."""

from __future__ import annotations

import json
import uuid
from abc import ABC, abstractmethod
from datetime import datetime, timezone
from typing import Any


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def new_event(
    *,
    tenant_id: str | None,
    workspace_id: str | None,
    event_type: str,
    subject: str,
    data: dict[str, Any],
) -> dict[str, Any]:
    return {
        "specversion": "1.0",
        "id": str(uuid.uuid4()),
        "source": "forge://service/diagnosis",
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
    def emit(self, event: dict[str, Any]) -> None: ...


class MemorySink(Sink):
    def __init__(self) -> None:
        self.events: list[dict[str, Any]] = []

    def emit(self, event: dict[str, Any]) -> None:
        self.events.append(event)


class LogSink(Sink):
    def emit(self, event: dict[str, Any]) -> None:
        print("event", json.dumps(event))
