from __future__ import annotations

from typing import Any
from uuid import uuid4

import httpx


class LiteLLMClient:
    def __init__(self, base_url: str, api_key: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key

    async def chat_completion(
        self,
        *,
        model: str,
        messages: list[dict[str, Any]],
        tenant_id: str = "local-tenant",
        workspace_id: str = "local-workspace",
        correlation_id: str | None = None,
        data_classification: str = "internal",
    ) -> Any:
        cid = correlation_id or str(uuid4())
        headers = {
            "authorization": f"Bearer {self.api_key}",
            "forgetenantid": tenant_id,
            "forgeworkspaceid": workspace_id,
            "forgecorrelationid": cid,
            "data_classification": data_classification,
        }
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.post(
                f"{self.base_url}/v1/chat/completions",
                headers=headers,
                json={"model": model, "messages": messages},
            )
        response.raise_for_status()
        return response.json()
