"""LiteLLM tenant budget probe for agent-mode.

The probe is called by the executor before every LLM-bound step. On
``over_budget`` the session pauses with ``paused_for_budget`` until the budget
is refreshed (admin top-up or new period).
"""

from __future__ import annotations

import uuid
from typing import Any

import httpx

from alfred.logging import get_logger

log = get_logger(__name__)


class BudgetProbe:
    def __init__(self, litellm_url: str, key: str) -> None:
        self._base = litellm_url.rstrip("/")
        self._key = key

    async def probe(self, workspace_id: uuid.UUID) -> dict[str, Any]:
        """Probe budget for the workspace's tenant key.

        Returns ``{"status": "ok"|"over_budget", "remaining_usd": float|None}``.
        Defaults to ``ok`` on transport errors so a degraded budget service
        cannot deadlock running sessions; tenant-level pausing remains
        enforced by LiteLLM's own gate.
        """
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.get(
                    f"{self._base}/budget",
                    params={"workspace_id": str(workspace_id)},
                    headers={"Authorization": f"Bearer {self._key}"} if self._key else {},
                )
                if r.status_code != 200:
                    return {"status": "ok", "reason": f"probe http {r.status_code}"}
                payload = r.json()
                remaining = payload.get("remaining_usd")
                if remaining is not None and remaining <= 0:
                    return {"status": "over_budget", "remaining_usd": remaining}
                return {"status": "ok", "remaining_usd": remaining}
        except httpx.HTTPError as exc:
            log.warning("budget_probe_failed", error=str(exc))
            return {"status": "ok", "reason": "probe unreachable"}
