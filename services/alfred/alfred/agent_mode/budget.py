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
    def __init__(
        self,
        litellm_url: str,
        key: str,
        *,
        default_budget_usd: float = 10.0,
        budget_window_hours: int = 24,
    ) -> None:
        self._base = litellm_url.rstrip("/")
        self._key = key
        self._default_budget_usd = default_budget_usd
        self._budget_window_hours = budget_window_hours

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
                    params={
                        "workspace_id": str(workspace_id),
                        "default_budget_usd": self._default_budget_usd,
                        "window_hours": self._budget_window_hours,
                    },
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

    async def probe_fingerprint(self, fingerprint: str) -> dict[str, Any]:
        """Per-fingerprint LLM-spend aggregate for the last hour.

        Returns::

            {
              "fingerprint": str,
              "sessions_last_hour": int,
              "tokens_last_hour": int,
              "status": "ok" | "over_budget",
            }

        This is queried by the triager before spawning a session for a given
        fingerprint.  Transport errors default to ok so a degraded budget
        service cannot block triage.
        """
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.get(
                    f"{self._base}/budget/fingerprint",
                    params={"fingerprint": fingerprint},
                    headers={"Authorization": f"Bearer {self._key}"} if self._key else {},
                )
                if r.status_code != 200:
                    return {"fingerprint": fingerprint, "status": "ok"}
                payload = r.json()
                sessions = payload.get("sessions_last_hour", 0)
                tokens = payload.get("tokens_last_hour", 0)
                max_sessions = payload.get("max_sessions_per_hour", 10)
                status = "over_budget" if sessions >= max_sessions else "ok"
                return {
                    "fingerprint": fingerprint,
                    "sessions_last_hour": sessions,
                    "tokens_last_hour": tokens,
                    "status": status,
                }
        except httpx.HTTPError as exc:
            log.warning("budget_probe_fingerprint_failed", error=str(exc), fingerprint=fingerprint)
            return {"fingerprint": fingerprint, "status": "ok", "reason": "probe unreachable"}

    async def probe_hourly(self, workspace_id: uuid.UUID | None = None) -> dict[str, Any]:
        """Per-hour aggregate across all fingerprints for the triager's LLM budget.

        Returns::

            {
              "sessions_this_hour": int,
              "tokens_this_hour": int,
              "max_sessions_per_hour": int,
              "max_tokens_per_hour": int,
              "status": "ok" | "over_budget",
            }
        """
        try:
            params: dict[str, str] = {"window": "1h"}
            if workspace_id is not None:
                params["workspace_id"] = str(workspace_id)
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.get(
                    f"{self._base}/budget/hourly",
                    params=params,
                    headers={"Authorization": f"Bearer {self._key}"} if self._key else {},
                )
                if r.status_code != 200:
                    return {"status": "ok"}
                payload = r.json()
                sessions = payload.get("sessions_this_hour", 0)
                max_sessions = payload.get("max_sessions_per_hour", 60)
                tokens = payload.get("tokens_this_hour", 0)
                max_tokens = payload.get("max_tokens_per_hour", 1_000_000)
                over = sessions >= max_sessions or tokens >= max_tokens
                return {
                    "sessions_this_hour": sessions,
                    "tokens_this_hour": tokens,
                    "max_sessions_per_hour": max_sessions,
                    "max_tokens_per_hour": max_tokens,
                    "status": "over_budget" if over else "ok",
                }
        except httpx.HTTPError as exc:
            log.warning("budget_probe_hourly_failed", error=str(exc))
            return {"status": "ok", "reason": "probe unreachable"}
