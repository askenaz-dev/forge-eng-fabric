from __future__ import annotations

import base64
import hashlib
import json
import os
import time
import uuid
from dataclasses import dataclass, field
from typing import Any

from fastapi import HTTPException
from fastapi.responses import PlainTextResponse
import httpx

from forge_mcp import MCPServer, ToolRequest


@dataclass
class GuardrailDecision:
    allowed: bool
    rationale: str
    audit: dict[str, Any]


class JiraGuardrails:
    def __init__(self, workspace_project_map: dict[str, set[str]] | None = None) -> None:
        self.workspace_project_map = workspace_project_map or _parse_workspace_projects(
            os.getenv("FORGE_JIRA_WORKSPACE_PROJECTS", ""),
        )

    def check(self, request: ToolRequest) -> GuardrailDecision:
        project_key = _project_from_params(request.params)
        audit = {
            "tool_id": request.tool_id,
            "workspace_id": request.context.workspace_id,
            "principal": request.context.principal,
            "correlation_id": request.context.correlation_id,
            "project_key": project_key,
        }
        if request.context.workspace_id and request.context.workspace_id in self.workspace_project_map:
            allowed = self.workspace_project_map[request.context.workspace_id]
            if project_key and project_key not in allowed:
                return GuardrailDecision(
                    False,
                    "project_not_mapped",
                    {**audit, "reason": "jira_project_unmapped", "allowed_projects": sorted(allowed)},
                )
        return GuardrailDecision(True, "allowed", {**audit, "reason": "jira_project_allowed"})


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
        kind = str(auth.get("type") or auth.get("kind") or "api_token")
        if kind not in {"api_token", "oauth2"}:
            raise ValueError("unsupported_jira_auth_type")
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
            id=f"jira-cred-{uuid.uuid4().hex[:12]}",
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
        super().__init__("jira_rate_limited")
        self.retry_after_seconds = retry_after_seconds


class CircuitOpenError(Exception):
    def __init__(self, retry_after_seconds: int) -> None:
        super().__init__("jira_circuit_open")
        self.retry_after_seconds = retry_after_seconds


class RateLimitAwareJiraClient:
    def __init__(
        self,
        failure_threshold: int = 3,
        circuit_seconds: int = 30,
        backend: str | None = None,
        base_url: str | None = None,
    ) -> None:
        self.failure_threshold = failure_threshold
        self.circuit_seconds = circuit_seconds
        self.failures = 0
        self.circuit_open_until = 0.0
        self.backend = backend or os.getenv("FORGE_JIRA_BACKEND", "stub")
        self.base_url = (base_url or os.getenv("FORGE_JIRA_BASE_URL", "")).rstrip("/")

    def call(self, operation: str, params: dict[str, Any], auth: dict[str, Any] | None = None) -> dict[str, Any]:
        now = time.time()
        if now < self.circuit_open_until:
            raise CircuitOpenError(max(1, int(self.circuit_open_until - now)))
        if self.backend == "atlassian":
            if not self.base_url:
                raise RuntimeError("jira_base_url_required")
            return self._call_atlassian(operation, params, auth)
        status = int(params.get("_simulate_status") or 200)
        if status == 429:
            retry_after = int(params.get("_retry_after_seconds") or 1)
            raise RateLimitedError(retry_after)
        if status >= 500:
            self.failures += 1
            if self.failures >= self.failure_threshold:
                self.circuit_open_until = now + self.circuit_seconds
            raise RuntimeError("jira_upstream_error")
        self.failures = 0
        return {"operation": operation, "status": "ok"}

    def _call_atlassian(self, operation: str, params: dict[str, Any], auth: dict[str, Any] | None) -> dict[str, Any]:
        headers = {"accept": "application/json", "content-type": "application/json"}
        client_auth: httpx.BasicAuth | None = None
        if auth:
            if auth.get("type") == "oauth2":
                headers["authorization"] = f"Bearer {auth.get('access_token')}"
            else:
                email = str(auth.get("email") or "")
                token = str(auth.get("api_token") or auth.get("token") or "")
                if not email or not token:
                    raise RuntimeError("jira_api_token_auth_requires_email_and_token")
                client_auth = httpx.BasicAuth(email, token)

        with httpx.Client(base_url=self.base_url, auth=client_auth, headers=headers, timeout=20.0) as client:
            if operation in {"create_issue", "create_epic"}:
                issue_type = str(params.get("issue_type") or ("Epic" if operation == "create_epic" else "Task"))
                response = client.post(
                    "/rest/api/3/issue",
                    json={
                        "fields": {
                            "project": {"key": str(params.get("project_key") or _project_from_params(params))},
                            "summary": str(params.get("summary") or "Forge MCP E2E issue"),
                            "description": _adf_text(str(params.get("description") or "Created by Forge Jira MCP E2E.")),
                            "issuetype": {"name": issue_type},
                        },
                    },
                )
            elif operation == "update_issue":
                response = client.put(
                    f"/rest/api/3/issue/{params['key']}",
                    json={"fields": params.get("fields") or {}},
                )
            elif operation == "transition_issue":
                transitions = client.get(f"/rest/api/3/issue/{params['key']}/transitions")
                self._raise_for_status(transitions)
                transition_id = _transition_id(transitions.json(), str(params.get("status") or params.get("transition") or ""))
                if not transition_id:
                    return {"operation": operation, "status": "skipped", "reason": "transition_not_available"}
                response = client.post(
                    f"/rest/api/3/issue/{params['key']}/transitions",
                    json={"transition": {"id": transition_id}},
                )
            elif operation == "add_comment":
                response = client.post(
                    f"/rest/api/3/issue/{params['key']}/comment",
                    json={"body": _adf_text(str(params.get("comment") or params.get("body") or ""))},
                )
            elif operation == "link_issue":
                response = client.post(
                    "/rest/api/3/issueLink",
                    json={
                        "type": {"name": str(params.get("link_type") or "Relates")},
                        "inwardIssue": {"key": str(params.get("inward_key") or params.get("key"))},
                        "outwardIssue": {"key": str(params.get("outward_key"))},
                    },
                )
            elif operation == "search":
                response = client.get("/rest/api/3/search", params={"jql": str(params.get("jql") or ""), "maxResults": 10})
            elif operation == "list_sprints":
                response = client.get(f"/rest/agile/1.0/board/{params['board_id']}/sprint")
            else:
                raise RuntimeError(f"unsupported_jira_operation: {operation}")

        self._raise_for_status(response)
        data = response.json() if response.content else {}
        return {"operation": operation, "status": "ok", "jira": data}

    def _raise_for_status(self, response: httpx.Response) -> None:
        if response.status_code == 429:
            retry_after = int(response.headers.get("retry-after") or "1")
            raise RateLimitedError(retry_after)
        if response.status_code >= 500:
            self.failures += 1
            if self.failures >= self.failure_threshold:
                self.circuit_open_until = time.time() + self.circuit_seconds
        response.raise_for_status()
        self.failures = 0


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


class JiraReconciler:
    def __init__(self, event_bus: MemoryEventBus) -> None:
        self.event_bus = event_bus

    def run_once(self, linked_issues: list[dict[str, Any]]) -> dict[str, Any]:
        reconciled = 0
        drift = 0
        for item in linked_issues:
            key = str(item.get("jira_key") or item.get("key") or "")
            issue = _issues.get(key)
            if not issue:
                drift += 1
                self.event_bus.emit("jira.issue.drift_detected.v1", {**item, "reason": "issue_missing"})
                continue
            expected_status = item.get("expected_status")
            if expected_status and issue.get("status") != expected_status:
                drift += 1
                self.event_bus.emit(
                    "jira.issue.drift_detected.v1",
                    {**item, "actual_status": issue.get("status"), "reason": "status_mismatch"},
                )
                continue
            reconciled += 1
            self.event_bus.emit("jira.issue.reconciled.v1", {**item, "status": issue.get("status")})
        return {"reconciled": reconciled, "drift": drift, "interval_seconds": 900}


def policy_hook(request: ToolRequest) -> tuple[bool, str]:
    if request.tool_id in {"mcp:jira.search", "mcp:jira.list_sprints", "mcp:jira.issue.read"}:
        return True, "read_only_allowed"
    decision = _guardrails.check(request)
    if not decision.allowed:
        _event_bus.emit("guardrail.trip.v1", decision.audit, request)
    return decision.allowed, decision.rationale


server = MCPServer(name="jira", policy_hook=policy_hook)


def _jira_call(operation: str, request: ToolRequest) -> dict[str, Any]:
    try:
        auth = request.params.get("auth")
        credential = _credential_store.store_from_request(auth if isinstance(auth, dict) else None)
        call = _jira_client.call(operation, request.params, auth if isinstance(auth, dict) else None)
        if credential:
            call["credential_ref"] = credential.id
            call["auth_type"] = credential.kind
        return call
    except RateLimitedError as exc:
        return {"rate_limited": True, "backoff_seconds": exc.retry_after_seconds}
    except CircuitOpenError as exc:
        return {"circuit_open": True, "retry_after_seconds": exc.retry_after_seconds}


@server.tool("mcp:jira.create_issue")
async def create_issue(request: ToolRequest) -> dict[str, object]:
    project_key = str(request.params.get("project_key") or _project_from_params(request.params) or "LOCAL")
    key = _next_issue_key(project_key)
    call = _jira_call("create_issue", request)
    jira_payload = call.get("jira") if isinstance(call.get("jira"), dict) else {}
    key = str(jira_payload.get("key") or key)
    issue = {
        "key": key,
        "project_key": project_key,
        "issue_type": str(request.params.get("issue_type") or "Task"),
        "summary": str(request.params.get("summary") or "Untitled issue"),
        "description": str(request.params.get("description") or ""),
        "status": str(request.params.get("status") or "To Do"),
        "openspec_id": request.params.get("openspec_id"),
    }
    _issues[key] = issue
    _event_bus.emit("jira.issue.created.v1", issue, request)
    return {**issue, **call}


@server.tool("mcp:jira.update_issue")
async def update_issue(request: ToolRequest) -> dict[str, object]:
    key = str(request.params.get("key") or "")
    issue = _issues.setdefault(key, {"key": key, "project_key": _project_from_key(key), "status": "To Do"})
    fields = request.params.get("fields") or {}
    if isinstance(fields, dict):
        issue.update(fields)
    call = _jira_call("update_issue", request)
    _event_bus.emit("jira.issue.updated.v1", issue, request)
    return {"key": key, "updated": True, "issue": issue, **call}


@server.tool("mcp:jira.transition_issue")
async def transition_issue(request: ToolRequest) -> dict[str, object]:
    key = str(request.params.get("key") or "")
    status = str(request.params.get("status") or request.params.get("transition") or "In Progress")
    issue = _issues.setdefault(key, {"key": key, "project_key": _project_from_key(key)})
    issue["status"] = status
    call = _jira_call("transition_issue", request)
    _event_bus.emit("jira.issue.updated.v1", issue, request)
    return {"key": key, "status": status, "transitioned": True, **call}


@server.tool("mcp:jira.add_comment")
async def add_comment(request: ToolRequest) -> dict[str, object]:
    key = str(request.params.get("key") or "")
    comment = str(request.params.get("comment") or request.params.get("body") or "")
    _comments.setdefault(key, []).append(comment)
    call = _jira_call("add_comment", request)
    _event_bus.emit("jira.issue.updated.v1", {"key": key, "comment_added": True}, request)
    return {"key": key, "comment_count": len(_comments[key]), **call}


@server.tool("mcp:jira.link_issue")
async def link_issue(request: ToolRequest) -> dict[str, object]:
    link = {
        "inward_key": request.params.get("inward_key") or request.params.get("key"),
        "outward_key": request.params.get("outward_key"),
        "link_type": request.params.get("link_type") or "relates_to",
    }
    _links.append(link)
    call = _jira_call("link_issue", request)
    return {"linked": True, "link": link, **call}


@server.tool("mcp:jira.create_epic")
async def create_epic(request: ToolRequest) -> dict[str, object]:
    params = dict(request.params)
    params["issue_type"] = "Epic"
    epic_request = request.model_copy(update={"params": params})
    result = await create_issue(epic_request)
    result["epic_key"] = result["key"]
    return result


@server.tool("mcp:jira.list_sprints")
async def list_sprints(request: ToolRequest) -> dict[str, object]:
    board_id = str(request.params.get("board_id") or "local-board")
    return {
        "board_id": board_id,
        "sprints": [
            {"id": f"{board_id}-sprint-1", "name": "Current Sprint", "state": "active"},
            {"id": f"{board_id}-sprint-2", "name": "Next Sprint", "state": "future"},
        ],
    }


@server.tool("mcp:jira.search")
async def search(request: ToolRequest) -> dict[str, object]:
    query = str(request.params.get("jql") or request.params.get("query") or "")
    issues = list(_issues.values())
    if query:
        issues = [issue for issue in issues if query.lower() in json.dumps(issue).lower()]
    return {"jql": query, "issues": issues}


@server.tool("mcp:jira.issue.read")
async def read_issue(request: ToolRequest) -> dict[str, object]:
    key = str(request.params.get("key") or "")
    return _issues.get(key, {"key": key, "fields": {}})


@server.tool("mcp:jira.issue.write")
async def write_issue(request: ToolRequest) -> dict[str, object]:
    if request.params.get("key"):
        return await update_issue(request)
    return await create_issue(request)


@server.tool("mcp:jira.sprint.update")
async def update_sprint(request: ToolRequest) -> dict[str, object]:
    return {"sprint": request.params.get("sprint"), "status": request.params.get("status")}


@server.app.post("/v1/webhooks/jira")
async def jira_webhook(payload: dict[str, Any]) -> dict[str, Any]:
    issue = payload.get("issue") or {}
    key = str(issue.get("key") or payload.get("key") or "")
    if not key:
        raise HTTPException(status_code=400, detail="jira_issue_key_required")
    fields = issue.get("fields") or {}
    status = fields.get("status") or {}
    event_data = {
        "key": key,
        "project_key": _project_from_key(key),
        "status": status.get("name") if isinstance(status, dict) else status,
        "webhook_event": payload.get("webhookEvent") or payload.get("event"),
        "openspec_id": payload.get("openspec_id") or fields.get("openspec_id"),
    }
    event_type = "jira.issue.updated.v1"
    if str(event_data["webhook_event"]).endswith("created"):
        event_type = "jira.issue.created.v1"
    _event_bus.emit(event_type, event_data)
    _notify_openspec_jira(event_data)
    return {"emitted": event_type, "key": key}


@server.app.post("/v1/reconcile")
async def reconcile(payload: dict[str, Any]) -> dict[str, Any]:
    linked_issues = payload.get("linked_issues") or []
    if not isinstance(linked_issues, list):
        raise HTTPException(status_code=400, detail="linked_issues_must_be_list")
    return _reconciler.run_once(linked_issues)


@server.app.get("/v1/events")
async def events() -> dict[str, Any]:
    return {"events": _event_bus.events}


@server.app.get("/metrics", response_class=PlainTextResponse)
async def metrics() -> str:
    return "# HELP jira_sync_lag_seconds Jira webhook/reconciliation sync lag in seconds.\n# TYPE jira_sync_lag_seconds gauge\njira_sync_lag_seconds 0\n"


def _parse_workspace_projects(raw: str) -> dict[str, set[str]]:
    result: dict[str, set[str]] = {}
    for item in raw.split(";"):
        if not item.strip() or "=" not in item:
            continue
        workspace, projects = item.split("=", 1)
        result[workspace.strip()] = {project.strip() for project in projects.split(",") if project.strip()}
    return result


def _project_from_params(params: dict[str, Any]) -> str:
    if params.get("project_key"):
        return str(params["project_key"])
    for key_name in ("key", "issue_key", "inward_key", "outward_key"):
        key = str(params.get(key_name) or "")
        if "-" in key:
            return _project_from_key(key)
    return ""


def _project_from_key(key: str) -> str:
    return key.split("-", 1)[0] if "-" in key else ""


def _next_issue_key(project_key: str) -> str:
    _sequence_by_project[project_key] = _sequence_by_project.get(project_key, 0) + 1
    return f"{project_key}-{_sequence_by_project[project_key]}"


def _notify_openspec_jira(event_data: dict[str, Any]) -> None:
    openspec_id = str(event_data.get("openspec_id") or "")
    if not openspec_id:
        return
    openspec_url = os.getenv("OPENSPEC_URL", "http://localhost:8083").rstrip("/")
    try:
        httpx.post(
            f"{openspec_url}/v1/hooks/jira",
            json={
                "openspec_id": openspec_id,
                "key": event_data.get("key"),
                "url": event_data.get("url"),
                "status": event_data.get("status"),
                "actor": "jira",
            },
            timeout=2.0,
        )
    except Exception:
        return


def _adf_text(text: str) -> dict[str, Any]:
    return {
        "type": "doc",
        "version": 1,
        "content": [{"type": "paragraph", "content": [{"type": "text", "text": text or " "}]}],
    }


def _transition_id(payload: dict[str, Any], name: str) -> str | None:
    if not name:
        return None
    for transition in payload.get("transitions") or []:
        if str(transition.get("name") or "").lower() == name.lower():
            return str(transition.get("id"))
    return None


_guardrails = JiraGuardrails()
_credential_store = EncryptedCredentialStore()
_jira_client = RateLimitAwareJiraClient()
_event_bus = MemoryEventBus()
_reconciler = JiraReconciler(_event_bus)
_issues: dict[str, dict[str, Any]] = {}
_comments: dict[str, list[str]] = {}
_links: list[dict[str, Any]] = []
_sequence_by_project: dict[str, int] = {}


app = server.app
