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
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry import trace

from .llm import LiteLLMClient

CONTROL_PLANE = os.getenv("CONTROL_PLANE_URL", "http://localhost:8081")
LITELLM_URL = os.getenv("LITELLM_URL", "http://localhost:4000")
LITELLM_KEY = os.getenv("LITELLM_KEY", "sk-forge-local")

app = FastAPI(title="Alfred stub", version="0.1.0")
llm_client = LiteLLMClient(LITELLM_URL, LITELLM_KEY)


def _configure_otel() -> None:
    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
    resource = Resource.create(
        {
            "service.name": "alfred-stub",
            "deployment.environment": os.getenv("ENV", "local"),
        }
    )
    provider = TracerProvider(resource=resource)
    provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(endpoint=endpoint, insecure=True)))
    trace.set_tracer_provider(provider)
    FastAPIInstrumentor.instrument_app(app)
    HTTPXClientInstrumentor().instrument()


_configure_otel()


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
    try:
        return await llm_client.chat_completion(
            model=body.get("model", "stub-chat"),
            messages=body.get("messages", [{"role": "user", "content": "hello"}]),
            tenant_id=body.get("tenant_id", "local-tenant"),
            workspace_id=body.get("workspace_id", "local-workspace"),
            correlation_id=body.get("correlation_id"),
            data_classification=body.get("data_classification", "internal"),
        )
    except httpx.HTTPStatusError as exc:
        raise HTTPException(status_code=exc.response.status_code, detail=exc.response.text) from exc
