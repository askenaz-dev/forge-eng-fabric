"""FastAPI host for Alfred, the Forge Control Plane Agent."""

from __future__ import annotations

import contextlib
import uuid
from contextlib import asynccontextmanager
from typing import Annotated, Any

from fastapi import Depends, FastAPI, Header, HTTPException, Query, Request, Response
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

from alfred.auth import Principal, fga_check, verify_jwt
from alfred.config import Settings, load_settings
from alfred.gateways import ApprovalsClient, OpenSpecClient, PermissionsClient, PolicyClient, RAGClient
from alfred.guardrails import Guardrails, GuardrailTrip
from alfred.llm import LiteLLMClient
from alfred.logging import configure_logging, get_logger
from alfred.loop import LoopDeps, run_intent
from alfred.models import IntentRequest, MessageIn
from alfred.observability import LangfuseObserver
from alfred.store import Store
from alfred.telemetry import init_tracing
from alfred.tools import ToolRouter

log = get_logger(__name__)


def create_app(
    *,
    settings: Settings | None = None,
    store: Store | None = None,
    loop_deps: LoopDeps | None = None,
    auth_required: bool = True,
) -> FastAPI:
    settings = settings or load_settings()
    configure_logging(settings.log_level)
    init_tracing("alfred", settings.otlp_endpoint)

    owned_store = store or Store(settings.postgres_url)
    deps = loop_deps or _build_loop_deps(settings, owned_store)

    @asynccontextmanager
    async def lifespan(_: FastAPI):
        await owned_store.connect()
        try:
            yield
        finally:
            await owned_store.close()

    app = FastAPI(title="Alfred", version="0.1.0", lifespan=lifespan)
    app.state.settings = settings
    app.state.store = owned_store
    app.state.loop_deps = deps
    app.state.auth_required = auth_required

    with contextlib.suppress(Exception):
        FastAPIInstrumentor.instrument_app(app)
        HTTPXClientInstrumentor().instrument()

    @app.middleware("http")
    async def correlation_middleware(request: Request, call_next):
        correlation_id = request.headers.get("X-Correlation-Id") or str(uuid.uuid4())
        request.state.correlation_id = correlation_id
        response = await call_next(request)
        response.headers["X-Correlation-Id"] = correlation_id
        return response

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/readyz")
    async def readyz() -> dict[str, str]:
        await owned_store.ping()
        return {"status": "ready"}

    @app.post("/v1/intents")
    async def submit_intent(
        request: Request,
        body: IntentRequest,
        response: Response,
        principal: Annotated[Principal, Depends(_principal)],
    ):
        await _require_workspace(request, principal, body.workspace_id, "can_edit")
        correlation_id = body.correlation_id or request.state.correlation_id
        response.headers["X-Correlation-Id"] = correlation_id
        return await run_intent(
            request.app.state.loop_deps,
            actor=principal.sub,
            workspace_id=body.workspace_id,
            intent=body.text,
            correlation_id=correlation_id,
            openspec_id=body.openspec_id,
            metadata=body.metadata,
        )

    @app.get("/v1/sessions/{session_id}")
    async def get_session(
        request: Request,
        session_id: uuid.UUID,
        principal: Annotated[Principal, Depends(_principal)],
    ) -> dict[str, Any]:
        session = await owned_store.get_session(session_id)
        if not session:
            raise HTTPException(status_code=404, detail="session not found")
        await _require_workspace(request, principal, session.workspace_id, "can_view")
        return {"session": session, "messages": await owned_store.list_messages(session_id)}

    @app.post("/v1/sessions/{session_id}/messages")
    async def append_message(
        request: Request,
        session_id: uuid.UUID,
        body: MessageIn,
        principal: Annotated[Principal, Depends(_principal)],
    ) -> dict[str, Any]:
        session = await owned_store.get_session(session_id)
        if not session:
            raise HTTPException(status_code=404, detail="session not found")
        await _require_workspace(request, principal, session.workspace_id, "can_edit")
        message_id = await owned_store.append_message(
            session_id=session_id,
            role=body.role,
            content=body.content,
            tool_call_id=body.tool_call_id,
        )
        return {"id": message_id, "session_id": session_id}

    @app.get("/v1/decisions")
    async def list_decisions(
        request: Request,
        principal: Annotated[Principal, Depends(_principal)],
        workspace_id: Annotated[uuid.UUID | None, Query()] = None,
        session_id: Annotated[uuid.UUID | None, Query()] = None,
        correlation_id: Annotated[str | None, Query()] = None,
        limit: Annotated[int, Query(ge=1, le=500)] = 100,
    ) -> dict[str, Any]:
        if workspace_id:
            await _require_workspace(request, principal, workspace_id, "can_view")
        elif session_id:
            session = await owned_store.get_session(session_id)
            if not session:
                raise HTTPException(status_code=404, detail="session not found")
            await _require_workspace(request, principal, session.workspace_id, "can_view")
        elif "platform-admin" not in principal.roles and request.app.state.auth_required:
            raise HTTPException(status_code=400, detail="workspace_id or session_id is required")
        decisions = await owned_store.list_decisions(
            workspace_id=workspace_id,
            session_id=session_id,
            correlation_id=correlation_id,
            limit=limit,
        )
        return {"decisions": decisions}

    return app


async def _principal(
    request: Request,
    authorization: Annotated[str | None, Header(alias="Authorization")] = None,
    dev_principal: Annotated[str | None, Header(alias="X-Forge-Dev-Principal")] = None,
) -> Principal:
    if not request.app.state.auth_required:
        sub = dev_principal or "dev-user"
        return Principal(sub=sub, email=None, name=sub, roles=("platform-admin",), raw={"dev": True})
    if not authorization or not authorization.lower().startswith("bearer "):
        raise HTTPException(status_code=401, detail="missing bearer token")
    token = authorization.split(" ", 1)[1].strip()
    settings: Settings = request.app.state.settings
    try:
        return verify_jwt(token, settings.keycloak_issuer, settings.keycloak_audience)
    except Exception as exc:
        raise HTTPException(status_code=401, detail=f"invalid bearer token: {exc}") from exc


async def _require_workspace(
    request: Request,
    principal: Principal,
    workspace_id: uuid.UUID,
    relation: str,
) -> None:
    settings: Settings = request.app.state.settings
    ok = await fga_check(
        base_url=settings.openfga_url,
        store_id=settings.openfga_store,
        model_id=settings.openfga_model,
        user=f"user:{principal.sub}",
        relation=relation,
        object_=f"workspace:{workspace_id}",
    )
    if not ok:
        raise HTTPException(status_code=403, detail=f"workspace {relation} required")


def _build_loop_deps(settings: Settings, store: Store) -> LoopDeps:
    guardrails = Guardrails(emit_trip=_emit_guardrail_trip)
    observer = None
    if settings.langfuse_public_key and settings.langfuse_secret_key:
        observer = LangfuseObserver(
            host=settings.langfuse_host,
            public_key=settings.langfuse_public_key,
            secret_key=settings.langfuse_secret_key,
        )
    tool_router = ToolRouter(
        mcp_endpoints={
            "github": settings.mcp_github_url,
            "jira": settings.mcp_jira_url,
            "confluence": settings.mcp_confluence_url,
            "openspec": settings.mcp_openspec_url,
        },
        skill_endpoint=settings.skill_runner_url,
        prompt_endpoint=settings.prompt_registry_url,
    )
    return LoopDeps(
        store=store,
        llm=LiteLLMClient(settings.litellm_url, settings.litellm_key, observer=observer),
        rag=RAGClient(settings.rag_query_url),
        policy=PolicyClient(settings.policy_engine_url),
        approvals=ApprovalsClient(settings.approvals_url),
        permissions=PermissionsClient(settings.permissions_url),
        openspec=OpenSpecClient(settings.openspec_url),
        guardrails=guardrails,
        tool_handler=tool_router.invoke,
        default_model=settings.default_model,
        rag_top_k=settings.rag_top_k,
        max_iterations=settings.max_loop_iterations,
        ai_observer=observer,
    )


def _emit_guardrail_trip(trip: GuardrailTrip) -> None:
    log.warning(
        "guardrail_trip",
        severity=trip.severity,
        pattern=trip.pattern,
        source=trip.source,
        detail=trip.detail,
    )


app = create_app()
