"""Minimal Alfred stub for Phase 0.

Exposes:
  GET  /healthz         -> liveness
  GET  /list-workspaces -> proxies the caller's token to the control-plane
  POST /chat            -> proxies a single message to LiteLLM

This is intentionally tiny; the real Alfred (planner / tool-calling / agentic
orchestration) lives in Phase 1+.
"""

from __future__ import annotations

import os
from typing import Any

import httpx
from fastapi import FastAPI, Header, HTTPException

CONTROL_PLANE = os.getenv("CONTROL_PLANE_URL", "http://localhost:8081")
LITELLM_URL = os.getenv("LITELLM_URL", "http://localhost:4000")
LITELLM_KEY = os.getenv("LITELLM_KEY", "sk-forge-local")

app = FastAPI(title="Alfred stub", version="0.1.0")


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/list-workspaces")
async def list_workspaces(authorization: str | None = Header(default=None)) -> Any:
    if not authorization:
        raise HTTPException(status_code=401, detail="missing authorization header")
    async with httpx.AsyncClient(timeout=10.0) as client:
        r = await client.get(
            f"{CONTROL_PLANE}/v1/workspaces",
            headers={"authorization": authorization},
        )
    if r.status_code >= 400:
        raise HTTPException(status_code=r.status_code, detail=r.text)
    return r.json()


@app.post("/chat")
async def chat(body: dict[str, Any]) -> Any:
    payload = {
        "model": body.get("model", "stub-chat"),
        "messages": body.get("messages", [{"role": "user", "content": "hello"}]),
    }
    async with httpx.AsyncClient(timeout=30.0) as client:
        r = await client.post(
            f"{LITELLM_URL}/v1/chat/completions",
            headers={"authorization": f"Bearer {LITELLM_KEY}"},
            json=payload,
        )
    if r.status_code >= 400:
        raise HTTPException(status_code=r.status_code, detail=r.text)
    return r.json()
