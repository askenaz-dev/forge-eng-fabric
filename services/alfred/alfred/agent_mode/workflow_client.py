"""Client for the workflow-runtime service.

Agent-mode dispatches the `forge.reference.intent-to-deploy@1` workflow through
this client and consumes the step events so they roll up into the session's
step rows.
"""

from __future__ import annotations

from typing import Any

import httpx

from alfred.logging import get_logger

log = get_logger(__name__)


class WorkflowRuntimeClient:
    def __init__(self, base_url: str) -> None:
        self._base = base_url.rstrip("/")

    async def dispatch(
        self, workflow_id: str, params: dict[str, Any], correlation_id: str
    ) -> dict[str, Any]:
        """Trigger a workflow run. Returns ``{"run_id": ..., "events": [...]}``."""
        try:
            async with httpx.AsyncClient(timeout=15.0) as client:
                r = await client.post(
                    f"{self._base}/v1/workflows/{workflow_id}/runs",
                    json={"params": params, "correlation_id": correlation_id},
                    headers={"X-Correlation-Id": correlation_id},
                )
                if r.status_code not in (200, 201, 202):
                    return {"run_id": None, "status": "error", "code": r.status_code}
                return r.json()
        except httpx.HTTPError as exc:
            log.warning("workflow_dispatch_failed", error=str(exc))
            return {"run_id": None, "status": "error", "error": str(exc)}

    async def get_run(self, run_id: str) -> dict[str, Any] | None:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.get(f"{self._base}/v1/workflow-runs/{run_id}")
                if r.status_code != 200:
                    return None
                return r.json()
        except httpx.HTTPError:
            return None
