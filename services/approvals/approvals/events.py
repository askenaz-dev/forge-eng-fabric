from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Protocol


class EventPublisher(Protocol):
    def publish(self, event_type: str, subject: str, data: dict[str, Any]) -> None: ...


@dataclass
class InMemoryEventPublisher:
    events: list[dict[str, Any]] = field(default_factory=list)

    def publish(self, event_type: str, subject: str, data: dict[str, Any]) -> None:
        self.events.append(
            {
                "specversion": "1.0",
                "id": str(uuid.uuid4()),
                "source": "forge.approvals",
                "type": event_type,
                "subject": subject,
                "time": datetime.utcnow().isoformat() + "Z",
                "data": data,
            }
        )
