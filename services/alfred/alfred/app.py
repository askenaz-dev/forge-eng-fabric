"""FastAPI host for Alfred, the Forge Control Plane Agent."""

from __future__ import annotations

import contextlib
import uuid
from contextlib import asynccontextmanager
from typing import Annotated, Any

from fastapi import Depends, FastAPI, Header, HTTPException, Query, Request, Response
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

from alfred.agent_mode.budget import BudgetProbe as AgentBudgetProbe
from alfred.agent_mode.events import EventEmitter as AgentEventEmitter, LogSink
from alfred.agent_mode.executor import ExecutorDeps as AgentExecutorDeps
from alfred.agent_mode.router import build_router as build_agent_mode_router
from alfred.agent_mode.store import AgentModeStore
from alfred.agent_mode.workflow_client import WorkflowRuntimeClient
from alfred.auth import Principal, fga_check, revoke_sub_principal, verify_jwt
from alfred.autonomy_presets import PresetStore
from alfred.config import Settings, load_settings
from alfred.dialogue import generate_followup
from alfred.gateways import ApprovalsClient, OpenSpecClient, PermissionsClient, PolicyClient, RAGClient
from alfred.guardrails import Guardrails, GuardrailTrip, _INJECTION_PAGE_THRESHOLD
from alfred.llm import LiteLLMClient, RequestContext
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
        tenant_id = _resolve_tenant_id(principal, body.tenant_id, request)
        return await run_intent(
            request.app.state.loop_deps,
            actor=principal.sub,
            workspace_id=body.workspace_id,
            intent=body.text,
            correlation_id=correlation_id,
            openspec_id=body.openspec_id,
            metadata=body.metadata,
            tenant_id=tenant_id,
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

    # Wizard dialogue API. Disabled by default; flip ALFRED_DIALOGUE_API=enabled
    # to surface the routes (per platform-gaps-closure task 3.11).
    dialogue_enabled = getattr(settings, "alfred_dialogue_api", "disabled").lower() == "enabled"

    # Phase 5 (app-first-class-entity 7.1): the App scope flag refuses intent
    # calls without an `app_id` once the workspace has cut over. Defaults to
    # `disabled` so legacy callers keep working during M0-M4 rollout.
    require_app_scope = getattr(settings, "alfred_require_app_scope", "disabled").lower() == "enabled"

    # alfred-console-redesign: dedup threshold config.
    _threshold_default = getattr(settings, "spec_match_threshold_default", 0.80)
    _threshold_floor = getattr(settings, "spec_match_threshold_floor", 0.65)

    # Shared in-memory event emitter for intent events (reuses agent-mode sink pattern).
    from alfred.agent_mode.events import EventEmitter as _EventEmitter, LogSink as _LogSink

    _intent_emitter = _EventEmitter(_LogSink())

    @app.post("/v1/intent/match")
    async def intent_match(
        request: Request,
        body: dict[str, Any],
        principal: Annotated[Principal, Depends(_principal)],
    ) -> dict[str, Any]:
        """RAG-based spec dedup retrieval (alfred-console-redesign 4.1-4.2).

        Accepts {workspace_id, app_id?, text, k?}.
        Returns {matches, threshold, scope}.
        Scores below the tenant threshold floor are filtered out.
        """
        if not dialogue_enabled:
            raise HTTPException(status_code=404, detail="dialogue API disabled")
        ws_id = body.get("workspace_id")
        if not ws_id:
            raise HTTPException(status_code=400, detail="workspace_id is required")
        await _require_workspace(request, principal, uuid.UUID(ws_id), "can_edit")

        text = body.get("text", "")
        if not text:
            raise HTTPException(status_code=400, detail="text is required")

        app_id = body.get("app_id")
        k = int(body.get("k", 5))

        # Tenant threshold (configurable via tenant config; uses service default for now).
        threshold = _threshold_default

        scope = f"app:{app_id}" if app_id else f"workspace:{ws_id}"

        hits = await deps.rag.query_with_app_scope(
            workspace_id=uuid.UUID(ws_id),
            text=text,
            top_k=k,
            principal=principal.sub,
            app_id=app_id,
        )

        # Normalise hits into the match schema; filter below floor.
        matches = []
        for h in hits:
            score = float(h.get("score", 0.0))
            if score < _threshold_floor:
                continue
            matches.append({
                "spec_id": h.get("spec_id") or h.get("id") or h.get("source_ref"),
                "title": h.get("title", ""),
                "score": score,
                "lifecycle_state": h.get("lifecycle_state", "proposed"),
                "summary": str(h.get("text", ""))[:280],
                "source": h.get("source_ref", ""),
            })

        return {"matches": matches, "threshold": threshold, "scope": scope}

    @app.post("/v1/intent/start")
    async def intent_start(
        request: Request,
        body: dict[str, Any],
        principal: Annotated[Principal, Depends(_principal)],
    ) -> dict[str, Any]:
        """Start an intent capture draft (alfred-console-redesign 5.1-5.2).

        New fields accepted:
          - view: "friendly" | "advanced"
          - bypass_match: bool (skip dedup pass)
          - resume_spec_id: str (extend an existing spec)
        """
        if not dialogue_enabled:
            raise HTTPException(status_code=404, detail="dialogue API disabled")
        ws_id = body.get("workspace_id")
        if not ws_id:
            raise HTTPException(status_code=400, detail="workspace_id is required")
        if require_app_scope and not body.get("app_id"):
            raise HTTPException(status_code=422, detail="missing_app_scope")
        await _require_workspace(request, principal, uuid.UUID(ws_id), "can_edit")
        body.setdefault("created_by", principal.sub)
        body.setdefault("correlation_id", request.state.correlation_id)

        view = body.get("view", "advanced")
        bypass_match = bool(body.get("bypass_match", False))
        resume_spec_id = body.get("resume_spec_id")

        # Dedup pass (alfred-console-redesign 5.2): run RAG retrieval before
        # persisting a draft, unless bypassed or resuming an existing spec.
        if not bypass_match and not resume_spec_id:
            business_intent = body.get("business_intent", "")
            app_id = body.get("app_id")
            hits = await deps.rag.query_with_app_scope(
                workspace_id=uuid.UUID(ws_id),
                text=business_intent,
                top_k=5,
                principal=principal.sub,
                app_id=app_id,
            )
            top_hit = next(
                (h for h in hits if float(h.get("score", 0.0)) >= _threshold_default), None
            )
            if top_hit:
                spec_match = {
                    "candidate": {
                        "spec_id": top_hit.get("spec_id") or top_hit.get("id"),
                        "title": top_hit.get("title", ""),
                        "score": float(top_hit.get("score", 0.0)),
                        "lifecycle_state": top_hit.get("lifecycle_state", "proposed"),
                        "summary": str(top_hit.get("text", ""))[:280],
                    },
                    "threshold": _threshold_default,
                }
                # Emit match_found event (5.5).
                await _intent_emitter.emit(
                    "alfred.intent.match_found.v1",
                    {
                        "principal": principal.sub,
                        "workspace_id": ws_id,
                        "app_id": app_id or "",
                        "spec_id": spec_match["candidate"]["spec_id"],
                        "score": spec_match["candidate"]["score"],
                        "threshold": _threshold_default,
                        "intent_text": business_intent[:280],
                        "view": view,
                        "correlation_id": body["correlation_id"],
                    },
                )
                # Return spec_match block — no draft persisted yet (5.2).
                return {"spec_match": spec_match, "view": view}

        start_body = {k: v for k, v in body.items() if k not in ("view", "bypass_match")}
        if resume_spec_id:
            start_body["resume_spec_id"] = resume_spec_id
        draft = await deps.openspec.start_intent(start_body)
        if not draft:
            raise HTTPException(status_code=502, detail="openspec service unreachable")
        completeness = await deps.openspec.completeness(draft["draft_id"])
        section, question = await generate_followup(
            completeness=completeness,
            turn_count=draft.get("turn_count", 0),
            last_answer="",
            correlation_id=draft.get("correlation_id"),
            llm=deps.llm,
            context=RequestContext(
                tenant_id=_resolve_tenant_id(principal, body.get("tenant_id"), request),
                workspace_id=ws_id,
                correlation_id=draft.get("correlation_id") or body["correlation_id"],
            ),
        )
        return {
            "draft": draft,
            "completeness": completeness,
            "next_question": question,
            "next_section": section,
            "view": view,
        }

    @app.get("/v1/intent/{draft_id}")
    async def intent_get(draft_id: str, principal: Annotated[Principal, Depends(_principal)]) -> dict[str, Any]:
        if not dialogue_enabled:
            raise HTTPException(status_code=404, detail="dialogue API disabled")
        draft = await deps.openspec.get_draft(draft_id)
        if not draft:
            raise HTTPException(status_code=404, detail="draft not found")
        completeness = await deps.openspec.completeness(draft_id)
        return {"draft": draft, "completeness": completeness}

    @app.post("/v1/intent/{draft_id}/answer")
    async def intent_answer(
        request: Request,
        draft_id: str,
        body: dict[str, Any],
        principal: Annotated[Principal, Depends(_principal)],
    ) -> dict[str, Any]:
        """Answer a wizard question (alfred-console-redesign 5.3-5.4).

        `view` is threaded through every turn and propagated to the LLM prompt
        and the audit event so Friendly persona rendering can be enforced.
        """
        if not dialogue_enabled:
            raise HTTPException(status_code=404, detail="dialogue API disabled")
        body.setdefault("actor", principal.sub)
        view = body.get("view", "advanced")
        # Phase 5: thread app_id through every answer so audit + decision-log
        # entries can carry the App scope (7.2).
        if "app_id" not in body:
            existing = await deps.openspec.get_draft(draft_id)
            if existing and existing.get("app_id"):
                body["app_id"] = existing["app_id"]
        if require_app_scope and not body.get("app_id"):
            raise HTTPException(status_code=422, detail="missing_app_scope")

        # Propagate view into the answer body so the OpenSpec service and the
        # LLM call can enforce friendly persona rendering (5.4).
        body["view"] = view

        updated = await deps.openspec.answer_intent(draft_id, body)
        if not updated:
            raise HTTPException(status_code=404, detail="draft not found or rejected")
        completeness = await deps.openspec.completeness(draft_id)
        ws_id_str = str(updated.get("workspace_id") or body.get("workspace_id") or "")
        section, question = await generate_followup(
            completeness=completeness,
            turn_count=updated.get("turn_count", 0),
            last_answer=body.get("answer", ""),
            correlation_id=updated.get("correlation_id"),
            llm=deps.llm,
            context=RequestContext(
                tenant_id=_resolve_tenant_id(principal, body.get("tenant_id"), request),
                workspace_id=ws_id_str,
                correlation_id=updated.get("correlation_id") or request.state.correlation_id,
            )
            if ws_id_str
            else None,
        )
        return {
            "draft": updated,
            "completeness": completeness,
            "next_question": question,
            "next_section": section,
            "view": view,
        }

    @app.post("/v1/intent/{draft_id}/commit")
    async def intent_commit(
        draft_id: str,
        body: dict[str, Any] | None,
        principal: Annotated[Principal, Depends(_principal)],
    ) -> dict[str, Any]:
        if not dialogue_enabled:
            raise HTTPException(status_code=404, detail="dialogue API disabled")
        payload = dict(body or {})
        payload.setdefault("actor", principal.sub)
        if require_app_scope:
            existing = await deps.openspec.get_draft(draft_id)
            if not existing or not existing.get("app_id"):
                raise HTTPException(status_code=422, detail="missing_app_scope")
        document = await deps.openspec.commit_intent(draft_id, payload)
        if not document:
            raise HTTPException(status_code=400, detail="draft not commit-ready")
        return {"openspec": document}

    @app.get("/v1/guardrails/injection-review")
    async def guardrail_injection_review(
        request: Request,
        principal: Annotated[Principal, Depends(_principal)],
    ) -> dict[str, Any]:
        """Admin endpoint: recent injection-pattern trips for manual review."""
        if "platform-admin" not in principal.roles and request.app.state.auth_required:
            raise HTTPException(status_code=403, detail="platform-admin role required")
        guardrails_obj: Guardrails = deps.guardrails
        queue = guardrails_obj.review_queue_snapshot()
        return {
            "trips": [t.cloud_event() for t in queue],
            "trips_last_hour": guardrails_obj.metrics.injection_trips_last_hour(),
            "page_threshold": _INJECTION_PAGE_THRESHOLD,
        }

    @app.get("/v1/guardrails/metrics")
    async def guardrail_metrics() -> Any:
        """Prometheus text exposition for guardrail counters."""
        from fastapi.responses import PlainTextResponse
        return PlainTextResponse(deps.guardrails.metrics.prometheus_text(), media_type="text/plain")

    @app.post("/v1/intent/match_dismissed")
    async def intent_match_dismissed(
        request: Request,
        body: dict[str, Any],
        principal: Annotated[Principal, Depends(_principal)],
    ) -> dict[str, Any]:
        """Record that the user dismissed a spec match (alfred-console-redesign 5.5).

        Emits alfred.intent.match_dismissed.v1 for retrieval relevance training.
        """
        if not dialogue_enabled:
            raise HTTPException(status_code=404, detail="dialogue API disabled")
        await _intent_emitter.emit(
            "alfred.intent.match_dismissed.v1",
            {
                "principal": principal.sub,
                "spec_id": body.get("spec_id", ""),
                "score": body.get("score", 0.0),
                "intent_text": str(body.get("intent_text", ""))[:280],
                "workspace_id": body.get("workspace_id", ""),
                "app_id": body.get("app_id", ""),
                "view": body.get("view", "advanced"),
                "correlation_id": request.state.correlation_id,
            },
        )
        return {"ok": True}

    if getattr(settings, "alfred_agent_mode_enabled", False):
        agent_store = AgentModeStore(owned_store)
        workflow_client = WorkflowRuntimeClient(settings.workflow_runtime_url)
        budget_probe = AgentBudgetProbe(
            settings.litellm_url,
            settings.litellm_key,
            default_budget_usd=settings.alfred_default_llm_budget_usd,
            budget_window_hours=settings.alfred_budget_window_hours,
        )
        event_emitter = AgentEventEmitter(LogSink())
        from pathlib import Path

        preset_store = PresetStore(root=Path(settings.agent_mode_preset_dir))
        async def _revoke_sub(session_id: str, workspace_id: str) -> None:
            await revoke_sub_principal(
                base_url=settings.openfga_url,
                store_id=settings.openfga_store,
                model_id=settings.openfga_model,
                session_id=session_id,
                workspace_id=workspace_id,
            )

        agent_deps = AgentExecutorDeps(
            store=owned_store,
            agent_store=agent_store,
            llm=deps.llm,
            rag=deps.rag,
            policy=deps.policy,
            approvals=deps.approvals,
            permissions=deps.permissions,
            openspec=deps.openspec,
            tool_handler=deps.tool_handler,
            workflow_dispatcher=workflow_client.dispatch,
            budget_probe=budget_probe.probe,
            emit_event=event_emitter.emit,
            ai_observer=deps.ai_observer,
            revoke_sub_principal=_revoke_sub,
            guardrails=deps.guardrails,
        )
        app.state.agent_mode = {
            "store": agent_store,
            "deps": agent_deps,
            "preset_store": preset_store,
            "emitter": event_emitter,
        }
        agent_router = build_agent_mode_router(
            settings=settings,
            agent_store=agent_store,
            executor_deps=agent_deps,
            preset_store=preset_store,
            event_emitter=event_emitter,
            principal_dep=_principal,
        )
        app.include_router(agent_router)
    else:
        @app.post("/v1/agent-mode/sessions")
        async def _agent_mode_disabled() -> dict[str, Any]:
            raise HTTPException(
                status_code=503, detail="agent-mode is disabled (ALFRED_AGENT_MODE_ENABLED=false)"
            )

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


def _resolve_tenant_id(
    principal: Principal,
    body_tenant_id: str | None,
    request: Request,
) -> str:
    """Resolve the tenant_id Alfred attaches to every LiteLLM call.

    alfred-litellm-header-injection (G1) requires a non-empty tenant on
    every outbound request. Order of resolution:
    1. explicit body field (`tenant_id`),
    2. JWT custom claim (`forge_tenant_id` or `tenant_id`),
    3. dev fallback `"dev-tenant"` when auth is not required.
    Returns `""` only in misconfigured prod, which causes LiteLLMClient
    to fail closed at call time with a clear error.
    """

    if body_tenant_id:
        return body_tenant_id
    claim = principal.raw.get("forge_tenant_id") or principal.raw.get("tenant_id")
    if isinstance(claim, str) and claim:
        return claim
    if not request.app.state.auth_required:
        return "dev-tenant"
    return ""


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
        # alfred-litellm-header-injection (G2): canonical prompt service.
        # `prompt:<id>:render` tools dispatch to prompt-template-service;
        # legacy prompt-registry is no longer wired here.
        prompt_template_service_url=settings.prompt_template_service_url,
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
