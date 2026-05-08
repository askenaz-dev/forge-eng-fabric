from __future__ import annotations

import base64
import hashlib
import json
import os
import time
import uuid
from dataclasses import dataclass, field
from typing import Any

import httpx
from fastapi import HTTPException
from fastapi.responses import PlainTextResponse

from forge_mcp import MCPServer, ToolRequest


@dataclass
class GuardrailDecision:
    allowed: bool
    rationale: str
    audit: dict[str, Any]


class ConfluenceGuardrails:
    def __init__(self, workspace_space_map: dict[str, set[str]] | None = None) -> None:
        self.workspace_space_map = workspace_space_map or _parse_workspace_spaces(
            os.getenv("FORGE_CONFLUENCE_WORKSPACE_SPACES", ""),
        )

    def check(self, request: ToolRequest) -> GuardrailDecision:
        space_key = _space_from_params(request.params)
        audit = {
            "tool_id": request.tool_id,
            "workspace_id": request.context.workspace_id,
            "principal": request.context.principal,
            "correlation_id": request.context.correlation_id,
            "space_key": space_key,
        }
        workspace_id = request.context.workspace_id
        if workspace_id and workspace_id in self.workspace_space_map:
            allowed = self.workspace_space_map[workspace_id]
            if space_key and space_key not in allowed:
                return GuardrailDecision(
                    False,
                    "space_not_mapped",
                    {**audit, "reason": "confluence_space_unmapped", "allowed_spaces": sorted(allowed)},
                )
        return GuardrailDecision(True, "allowed", {**audit, "reason": "confluence_space_allowed"})


@dataclass
class CredentialRecord:
    id: str
    kind: str
    ciphertext: str
    email: str | None = None
    scopes: list[str] = field(default_factory=list)
    created_at: float = field(default_factory=time.time)


class EncryptedCredentialStore:
    def __init__(self, master_key: str | None = None) -> None:
        self.master_key = master_key or os.getenv("FORGE_MCP_CREDENTIAL_KEY", "local-dev-key")
        self.records: dict[str, CredentialRecord] = {}

    def store_from_request(self, auth: dict[str, Any] | None) -> CredentialRecord | None:
        if not auth:
            return None
        kind = str(auth.get("type") or "api_token")
        if kind not in {"api_token", "oauth2"}:
            raise ValueError("unsupported_confluence_auth_type")
        secret_payload = {
            "api_token": auth.get("api_token") or auth.get("token"),
            "access_token": auth.get("access_token"),
            "refresh_token": auth.get("refresh_token"),
        }
        if kind == "api_token" and not secret_payload["api_token"]:
            raise ValueError("api_token_required")
        if kind == "oauth2" and not secret_payload["access_token"]:
            raise ValueError("access_token_required")
        record = CredentialRecord(
            id=f"confluence-cred-{uuid.uuid4().hex[:12]}",
            kind=kind,
            ciphertext=self._encrypt(json.dumps(secret_payload, sort_keys=True)),
            email=str(auth.get("email") or "") or None,
            scopes=list(auth.get("scopes") or []),
        )
        self.records[record.id] = record
        return record

    def _encrypt(self, plaintext: str) -> str:
        key = hashlib.sha256(self.master_key.encode("utf-8")).digest()
        raw = plaintext.encode("utf-8")
        encrypted = bytes(byte ^ key[index % len(key)] for index, byte in enumerate(raw))
        return base64.urlsafe_b64encode(encrypted).decode("ascii")


class RateLimitedError(Exception):
    def __init__(self, retry_after_seconds: int) -> None:
        super().__init__("confluence_rate_limited")
        self.retry_after_seconds = retry_after_seconds


class ConfluenceClient:
    def __init__(self, backend: str | None = None, base_url: str | None = None) -> None:
        self.backend = backend or os.getenv("FORGE_CONFLUENCE_BACKEND", "stub")
        self.base_url = (base_url or os.getenv("FORGE_CONFLUENCE_BASE_URL", "")).rstrip("/")

    def call(self, operation: str, params: dict[str, Any], auth: dict[str, Any] | None = None) -> dict[str, Any]:
        if self.backend != "atlassian":
            return {"operation": operation, "status": "ok"}
        if not self.base_url:
            raise RuntimeError("confluence_base_url_required")
        return self._call_atlassian(operation, params, auth)

    def _call_atlassian(self, operation: str, params: dict[str, Any], auth: dict[str, Any] | None) -> dict[str, Any]:
        headers = {"accept": "application/json"}
        client_auth: httpx.BasicAuth | None = None
        if auth:
            if auth.get("type") == "oauth2":
                headers["authorization"] = f"Bearer {auth.get('access_token')}"
            else:
                email = str(auth.get("email") or "")
                token = str(auth.get("api_token") or auth.get("token") or "")
                if not email or not token:
                    raise RuntimeError("confluence_api_token_auth_requires_email_and_token")
                client_auth = httpx.BasicAuth(email, token)
        with httpx.Client(base_url=self.base_url, auth=client_auth, headers=headers, timeout=20.0) as client:
            if operation == "create_page":
                response = client.post(
                    "/wiki/rest/api/content",
                    json={
                        "type": "page",
                        "title": str(params.get("title") or "Forge page"),
                        "space": {"key": str(params.get("space_key"))},
                        "body": {"storage": {"value": str(params.get("body") or ""), "representation": "storage"}},
                    },
                )
            elif operation == "update_page":
                page_id = str(params.get("page_id"))
                current = client.get(f"/wiki/rest/api/content/{page_id}", params={"expand": "version,space"})
                self._raise_for_status(current)
                current_data = current.json()
                response = client.put(
                    f"/wiki/rest/api/content/{page_id}",
                    json={
                        "id": page_id,
                        "type": "page",
                        "title": str(params.get("title") or current_data.get("title") or "Forge page"),
                        "space": {"key": str(params.get("space_key") or current_data.get("space", {}).get("key") or "")},
                        "body": {"storage": {"value": str(params.get("body") or ""), "representation": "storage"}},
                        "version": {"number": int(current_data.get("version", {}).get("number") or 1) + 1},
                    },
                )
            elif operation == "add_label":
                response = client.post(
                    f"/wiki/rest/api/content/{params['page_id']}/label",
                    json=[{"prefix": "global", "name": str(params.get("label") or "forge-managed")}],
                )
            elif operation == "attach_file":
                response = client.post(
                    f"/wiki/rest/api/content/{params['page_id']}/child/attachment",
                    headers={**headers, "X-Atlassian-Token": "no-check"},
                    files={"file": (str(params.get("filename") or "attachment.txt"), bytes(str(params.get("content") or ""), "utf-8"))},
                )
            elif operation == "search":
                response = client.get("/wiki/rest/api/content/search", params={"cql": str(params.get("cql") or ""), "limit": 10})
            else:
                raise RuntimeError(f"unsupported_confluence_operation: {operation}")
        self._raise_for_status(response)
        return {"operation": operation, "status": "ok", "confluence": response.json() if response.content else {}}

    def _raise_for_status(self, response: httpx.Response) -> None:
        if response.status_code == 429:
            retry_after = int(response.headers.get("retry-after") or "1")
            raise RateLimitedError(retry_after)
        response.raise_for_status()


class MemoryEventBus:
    def __init__(self) -> None:
        self.events: list[dict[str, Any]] = []

    def emit(self, event_type: str, data: dict[str, Any], context: ToolRequest | None = None) -> None:
        self.events.append(
            {
                "type": event_type,
                "data": data,
                "workspace_id": context.context.workspace_id if context else data.get("workspace_id"),
                "correlation_id": context.context.correlation_id if context else data.get("correlation_id"),
                "time": time.time(),
            },
        )


def policy_hook(request: ToolRequest) -> tuple[bool, str]:
    if request.tool_id in {"mcp:confluence.search", "mcp:confluence.page.read"}:
        return True, "read_only_allowed"
    decision = _guardrails.check(request)
    if not decision.allowed:
        _event_bus.emit("guardrail.trip.v1", decision.audit, request)
    return decision.allowed, decision.rationale


server = MCPServer(name="confluence", policy_hook=policy_hook)


def _confluence_call(operation: str, request: ToolRequest) -> dict[str, Any]:
    try:
        auth = request.params.get("auth")
        credential = _credential_store.store_from_request(auth if isinstance(auth, dict) else None)
        call = _confluence_client.call(operation, request.params, auth if isinstance(auth, dict) else None)
        if credential:
            call["credential_ref"] = credential.id
            call["auth_type"] = credential.kind
        return call
    except RateLimitedError as exc:
        return {"rate_limited": True, "backoff_seconds": exc.retry_after_seconds}


@server.tool("mcp:confluence.create_page")
async def create_page(request: ToolRequest) -> dict[str, object]:
    page_id = f"local-page-{uuid.uuid4().hex[:8]}"
    body = _managed_body(str(request.params.get("body") or ""), str(request.params.get("openspec_id") or ""))
    params = {**request.params, "body": body}
    call_request = request.model_copy(update={"params": params})
    call = _confluence_call("create_page", call_request)
    confluence_payload = call.get("confluence") if isinstance(call.get("confluence"), dict) else {}
    page_id = str(confluence_payload.get("id") or page_id)
    page = {
        "page_id": page_id,
        "space_key": str(request.params.get("space_key") or ""),
        "title": str(request.params.get("title") or "Untitled page"),
        "body": body,
        "labels": ["forge-managed"],
        "openspec_id": request.params.get("openspec_id"),
    }
    _pages[page_id] = page
    if _confluence_client.backend == "atlassian" and not call.get("rate_limited"):
        label_request = request.model_copy(update={"params": {**request.params, "page_id": page_id, "label": "forge-managed"}})
        _confluence_call("add_label", label_request)
    _event_bus.emit("confluence.page.created.v1", page, request)
    return {**page, **call}


@server.tool("mcp:confluence.update_page")
async def update_page(request: ToolRequest) -> dict[str, object]:
    page_id = str(request.params.get("page_id") or "")
    existing = _pages.setdefault(page_id, {"page_id": page_id, "labels": ["forge-managed"]})
    body = _managed_body(str(request.params.get("body") or existing.get("body") or ""), str(request.params.get("openspec_id") or existing.get("openspec_id") or ""))
    params = {**request.params, "body": body}
    call = _confluence_call("update_page", request.model_copy(update={"params": params}))
    existing.update({"title": request.params.get("title") or existing.get("title"), "body": body})
    _event_bus.emit("confluence.page.updated.v1", existing, request)
    return {"page_id": page_id, "updated": True, "page": existing, **call}


@server.tool("mcp:confluence.attach_file")
async def attach_file(request: ToolRequest) -> dict[str, object]:
    page_id = str(request.params.get("page_id") or "")
    filename = str(request.params.get("filename") or "attachment.txt")
    call = _confluence_call("attach_file", request)
    attachment = {"page_id": page_id, "filename": filename, "attached": True}
    _attachments.setdefault(page_id, []).append(attachment)
    _event_bus.emit("confluence.page.updated.v1", {"page_id": page_id, "attachment": filename}, request)
    return {**attachment, **call}


@server.tool("mcp:confluence.add_label")
async def add_label(request: ToolRequest) -> dict[str, object]:
    page_id = str(request.params.get("page_id") or "")
    label = str(request.params.get("label") or "forge-managed")
    call = _confluence_call("add_label", request)
    page = _pages.setdefault(page_id, {"page_id": page_id, "labels": []})
    labels = page.setdefault("labels", [])
    if label not in labels:
        labels.append(label)
    _event_bus.emit("confluence.page.updated.v1", {"page_id": page_id, "label": label}, request)
    return {"page_id": page_id, "label": label, "labels": labels, **call}


@server.tool("mcp:confluence.search")
async def search(request: ToolRequest) -> dict[str, object]:
    call = _confluence_call("search", request)
    query = str(request.params.get("query") or request.params.get("cql") or "")
    pages = list(_pages.values())
    if query:
        pages = [page for page in pages if query.lower() in json.dumps(page).lower()]
    return {"query": query, "pages": pages, **call}


@server.tool("mcp:confluence.page.read")
async def read_page(request: ToolRequest) -> dict[str, object]:
    page_id = str(request.params.get("page_id") or "")
    return _pages.get(page_id, {"page_id": page_id, "title": "", "body": ""})


@server.tool("mcp:confluence.page.write")
async def write_page(request: ToolRequest) -> dict[str, object]:
    if request.params.get("page_id"):
        return await update_page(request)
    return await create_page(request)


@server.app.post("/v1/webhooks/confluence")
async def confluence_webhook(payload: dict[str, Any]) -> dict[str, Any]:
    page = payload.get("page") or payload.get("content") or {}
    page_id = str(page.get("id") or payload.get("page_id") or "")
    if not page_id:
        raise HTTPException(status_code=400, detail="confluence_page_id_required")
    event_name = str(payload.get("event") or payload.get("webhookEvent") or "page_updated")
    event_type = "confluence.page.created.v1" if "created" in event_name else "confluence.page.updated.v1"
    event_data = {
        "page_id": page_id,
        "space_key": _space_from_params(page),
        "title": page.get("title"),
        "openspec_id": payload.get("openspec_id"),
    }
    _event_bus.emit(event_type, event_data)
    return {"emitted": event_type, "page_id": page_id}


@server.app.get("/v1/events")
async def events() -> dict[str, Any]:
    return {"events": _event_bus.events}


@server.app.get("/metrics", response_class=PlainTextResponse)
async def metrics() -> str:
    return "# HELP confluence_sync_lag_seconds Confluence webhook sync lag in seconds.\n# TYPE confluence_sync_lag_seconds gauge\nconfluence_sync_lag_seconds 0\n"


def _managed_body(body: str, openspec_id: str) -> str:
    header = '<p><strong>OpenSpec:</strong> {}</p><p><em>Editado por Alfred - cambios humanos posibles, ver historial.</em></p>'.format(
        openspec_id or "unlinked",
    )
    return f"{header}\n{body}"


def _parse_workspace_spaces(raw: str) -> dict[str, set[str]]:
    result: dict[str, set[str]] = {}
    for item in raw.split(";"):
        if not item.strip() or "=" not in item:
            continue
        workspace, spaces = item.split("=", 1)
        result[workspace.strip()] = {space.strip() for space in spaces.split(",") if space.strip()}
    return result


def _space_from_params(params: dict[str, Any]) -> str:
    if params.get("space_key"):
        return str(params["space_key"])
    space = params.get("space")
    if isinstance(space, dict):
        return str(space.get("key") or "")
    return ""


_guardrails = ConfluenceGuardrails()
_credential_store = EncryptedCredentialStore()
_confluence_client = ConfluenceClient()
_event_bus = MemoryEventBus()
_pages: dict[str, dict[str, Any]] = {}
_attachments: dict[str, list[dict[str, Any]]] = {}


app = server.app
