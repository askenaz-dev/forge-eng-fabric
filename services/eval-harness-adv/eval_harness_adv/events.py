"""CloudEvents emitted by the eval harness."""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Any

from pydantic import BaseModel, Field


class CloudEvent(BaseModel):
    specversion: str = "1.0"
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    source: str = "forge://service/eval-harness-adv"
    type: str
    subject: str | None = None
    time: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    datacontenttype: str = "application/json"
    forgetenantid: str | None = None
    forgeworkspaceid: str | None = None
    data: dict[str, Any]

    @classmethod
    def new(
        cls,
        *,
        type_: str,
        tenant_id: str | None = None,
        workspace_id: str | None = None,
        subject: str | None = None,
        data: dict[str, Any] | None = None,
    ) -> "CloudEvent":
        return cls(
            type=type_,
            forgetenantid=tenant_id,
            forgeworkspaceid=workspace_id,
            subject=subject,
            data=data or {},
        )


class EventSink:
    def emit(self, event: CloudEvent) -> None:  # pragma: no cover - interface
        raise NotImplementedError


class MemorySink(EventSink):
    def __init__(self) -> None:
        self.events: list[CloudEvent] = []

    def emit(self, event: CloudEvent) -> None:
        self.events.append(event)

    def by_type(self, type_: str) -> list[CloudEvent]:
        return [e for e in self.events if e.type == type_]


class LogSink(EventSink):
    def emit(self, event: CloudEvent) -> None:
        print("event", event.model_dump_json())
