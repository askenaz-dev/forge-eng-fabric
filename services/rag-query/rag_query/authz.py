from __future__ import annotations

import uuid

import httpx


async def can_view_workspace(
    *,
    openfga_url: str,
    store_id: str,
    model_id: str,
    principal: str,
    workspace_id: uuid.UUID,
) -> bool:
    if not store_id:
        return True
    payload: dict[str, object] = {
        "tuple_key": {
            "user": f"user:{principal}",
            "relation": "can_view",
            "object": f"workspace:{workspace_id}",
        }
    }
    if model_id:
        payload["authorization_model_id"] = model_id
    async with httpx.AsyncClient(timeout=5.0) as client:
        response = await client.post(f"{openfga_url.rstrip('/')}/stores/{store_id}/check", json=payload)
        if response.status_code != 200:
            return False
        return bool(response.json().get("allowed"))
