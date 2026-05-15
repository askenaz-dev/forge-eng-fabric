"""Dedup index lifecycle event handlers.

Reacts to OpenSpec lifecycle CloudEvents so the Milvus retrieval corpus stays
accurate. Called from the event-ingestion path (e.g. a NATS/Kafka consumer or
a webhook listener).

Supported events (alfred-console-redesign spec-deduplication spec):
- spec.purged.v1      → remove from index
- spec.reparented.v1  → update app_id metadata
- intent.committed.v1 → index if not already present
"""

from __future__ import annotations

from typing import Any

import httpx

from alfred.logging import get_logger

log = get_logger(__name__)


class DedupIndexClient:
    """Thin async client over the dedup / Milvus index REST API."""

    def __init__(self, base_url: str) -> None:
        self._base = base_url.rstrip("/")

    async def delete(self, spec_id: str) -> bool:
        try:
            async with httpx.AsyncClient(timeout=5.0) as c:
                r = await c.delete(f"{self._base}/v1/dedup/index/{spec_id}")
                return r.status_code in (200, 204, 404)
        except httpx.HTTPError as exc:
            log.warning("dedup_index_delete_failed", spec_id=spec_id, error=str(exc))
            return False

    async def update_app(self, spec_id: str, new_app_id: str) -> bool:
        try:
            async with httpx.AsyncClient(timeout=5.0) as c:
                r = await c.patch(
                    f"{self._base}/v1/dedup/index/{spec_id}",
                    json={"app_id": new_app_id},
                )
                return r.status_code in (200, 204)
        except httpx.HTTPError as exc:
            log.warning("dedup_index_update_failed", spec_id=spec_id, error=str(exc))
            return False

    async def upsert(self, spec: dict[str, Any]) -> bool:
        try:
            async with httpx.AsyncClient(timeout=5.0) as c:
                r = await c.post(f"{self._base}/v1/dedup/index", json=spec)
                return r.status_code in (200, 201)
        except httpx.HTTPError as exc:
            log.warning("dedup_index_upsert_failed", spec_id=spec.get("spec_id"), error=str(exc))
            return False


async def handle_spec_purged(event: dict[str, Any], client: DedupIndexClient) -> None:
    spec_id = (event.get("data") or {}).get("spec_id") or event.get("subject", "")
    if not spec_id:
        log.warning("dedup_handle_purged_missing_spec_id")
        return
    ok = await client.delete(spec_id)
    log.info("dedup_handle_purged", spec_id=spec_id, ok=ok)


async def handle_spec_reparented(event: dict[str, Any], client: DedupIndexClient) -> None:
    data = event.get("data") or {}
    spec_id = data.get("spec_id") or event.get("subject", "")
    new_app_id = data.get("to_app_id", "")
    if not spec_id or not new_app_id:
        log.warning("dedup_handle_reparented_missing_fields", data=data)
        return
    ok = await client.update_app(spec_id, new_app_id)
    log.info("dedup_handle_reparented", spec_id=spec_id, new_app_id=new_app_id, ok=ok)


async def handle_intent_committed(event: dict[str, Any], client: DedupIndexClient) -> None:
    data = event.get("data") or {}
    spec = {
        "spec_id": data.get("spec_id") or data.get("openspec_id"),
        "title": data.get("title", ""),
        "app_id": data.get("app_id"),
        "workspace_id": data.get("workspace_id"),
        "lifecycle_state": data.get("lifecycle_state", "committed"),
        "summary": data.get("summary", ""),
    }
    if not spec["spec_id"]:
        log.warning("dedup_handle_committed_missing_spec_id", data=data)
        return
    ok = await client.upsert(spec)
    log.info("dedup_handle_committed", spec_id=spec["spec_id"], ok=ok)
