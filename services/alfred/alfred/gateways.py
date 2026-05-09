"""Async clients for sibling services (RAG, policy, approvals, MCP, Skills, Prompts)."""

from __future__ import annotations

import uuid
from typing import Any

import httpx


class RAGClient:
    def __init__(self, base_url: str) -> None:
        self._base = base_url.rstrip("/")

    async def query(
        self,
        *,
        workspace_id: uuid.UUID,
        text: str,
        top_k: int = 8,
        principal: str = "alfred",
    ) -> list[dict[str, Any]]:
        try:
            async with httpx.AsyncClient(timeout=10.0) as client:
                r = await client.post(
                    f"{self._base}/v1/query",
                    json={
                        "workspace_id": str(workspace_id),
                        "text": text,
                        "top_k": top_k,
                        "principal": principal,
                    },
                )
                if r.status_code != 200:
                    return []
                return r.json().get("results", [])
        except httpx.HTTPError:
            return []


class PolicyClient:
    def __init__(self, base_url: str) -> None:
        self._base = base_url.rstrip("/")

    async def evaluate(
        self,
        *,
        principal: str,
        action: str,
        workspace_id: uuid.UUID,
        openspec_id: str | None,
        target: dict[str, Any],
        env: str = "dev",
        criticality: str = "low",
    ) -> dict[str, Any]:
        body = {
            "principal": principal,
            "action": action,
            "workspace_id": str(workspace_id),
            "openspec_id": openspec_id,
            "target": target,
            "env": env,
            "criticality": criticality,
        }
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.post(f"{self._base}/v1/evaluate", json=body)
                if r.status_code != 200:
                    return {"decision": "allow", "rationale": "policy-engine unavailable, default-allow (DEV)"}
                return r.json()
        except httpx.HTTPError:
            return {"decision": "allow", "rationale": "policy-engine unreachable, default-allow (DEV)"}


class ApprovalsClient:
    def __init__(self, base_url: str) -> None:
        self._base = base_url.rstrip("/")

    async def request(
        self,
        *,
        principal: str,
        action: str,
        workspace_id: uuid.UUID,
        openspec_id: str | None,
        target: dict[str, Any],
        rationale: str,
        required_approvers: list[str],
        criticality: str,
        correlation_id: str,
    ) -> dict[str, Any]:
        body = {
            "principal": principal,
            "action": action,
            "workspace_id": str(workspace_id),
            "openspec_id": openspec_id,
            "target": target,
            "rationale": rationale,
            "required_approvers": required_approvers,
            "criticality": criticality,
            "correlation_id": correlation_id,
        }
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.post(f"{self._base}/v1/approvals", json=body)
                r.raise_for_status()
                return r.json()
        except httpx.HTTPError as exc:
            return {"id": None, "status": "error", "error": str(exc)}


class PermissionsClient:
    def __init__(self, base_url: str) -> None:
        self._base = base_url.rstrip("/")

    async def can(
        self,
        *,
        subject: str,
        action_class: str,
        scope_kind: str,
        scope_id: str,
        criticality: str,
    ) -> dict[str, Any]:
        body = {
            "subject": subject,
            "action_class": action_class,
            "scope_kind": scope_kind,
            "scope_id": scope_id,
            "criticality": criticality,
        }
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.post(f"{self._base}/v1/permissions/check", json=body)
                if r.status_code != 200:
                    return {"allowed": False, "reason": f"permissions http {r.status_code}"}
                return r.json()
        except httpx.HTTPError as exc:
            return {"allowed": False, "reason": f"permissions error: {exc}"}


class OpenSpecClient:
    def __init__(self, base_url: str) -> None:
        self._base = base_url.rstrip("/")

    async def get(self, openspec_id: str) -> dict[str, Any] | None:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.get(f"{self._base}/v1/openspecs/{openspec_id}")
                if r.status_code != 200:
                    return None
                return r.json()
        except httpx.HTTPError:
            return None

    async def append_decision(self, openspec_id: str, decision: dict[str, Any]) -> bool:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.post(
                    f"{self._base}/v1/openspecs/{openspec_id}/decisions", json=decision
                )
                return r.status_code in (200, 201)
        except httpx.HTTPError:
            return False

    async def start_intent(self, body: dict[str, Any]) -> dict[str, Any] | None:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.post(f"{self._base}/v1/intent/start", json=body)
                if r.status_code != 201:
                    return None
                return r.json()
        except httpx.HTTPError:
            return None

    async def get_draft(self, draft_id: str) -> dict[str, Any] | None:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.get(f"{self._base}/v1/intent/{draft_id}")
                if r.status_code != 200:
                    return None
                return r.json()
        except httpx.HTTPError:
            return None

    async def answer_intent(self, draft_id: str, body: dict[str, Any]) -> dict[str, Any] | None:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.post(f"{self._base}/v1/intent/{draft_id}/answer", json=body)
                if r.status_code != 200:
                    return None
                return r.json()
        except httpx.HTTPError:
            return None

    async def commit_intent(self, draft_id: str, body: dict[str, Any]) -> dict[str, Any] | None:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.post(f"{self._base}/v1/intent/{draft_id}/commit", json=body)
                if r.status_code != 200:
                    return None
                return r.json()
        except httpx.HTTPError:
            return None

    async def completeness(self, draft_or_openspec_id: str) -> dict[str, Any] | None:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                r = await client.get(f"{self._base}/v1/openspecs/{draft_or_openspec_id}/completeness")
                if r.status_code != 200:
                    return None
                return r.json()
        except httpx.HTTPError:
            return None
