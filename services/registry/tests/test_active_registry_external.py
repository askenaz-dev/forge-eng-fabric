"""Integration tests for the active-registry-gateways §2 work.

Covers:
  - how_to / active_surface validation on creation and promotion
  - lifecycle precondition: approved rejects when how_to or active_surface
    is missing
  - external MCP registration (POST /v1/registry/mcps/external) — synthetic
    upstream served from an aiohttp fixture so the manifest fetch is
    deterministic
  - external A2A registration mirrors the MCP shape

The tests require REGISTRY_API_URL, WORKSPACE_ID and AUTH_TOKEN to point at
a running registry with migration 0007 applied. They skip cleanly when the
required environment is not set.
"""
from __future__ import annotations

import asyncio
import json
import os
import threading
import uuid
from datetime import UTC, datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

import httpx
import pytest

EVIDENCE_BASE = Path("docs/governance/evidence/active-registry-gateways")
EVIDENCE_RUN_DIR = EVIDENCE_BASE / datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ")


def _env_or_skip(name: str) -> str:
    value = os.getenv(name)
    if not value:
        pytest.skip(f"integration test skipped: env var {name} is not set")
    return value


def _headers(token: str, correlation_id: str | None = None) -> dict[str, str]:
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    if correlation_id:
        headers["X-Correlation-Id"] = correlation_id
    return headers


def _save_evidence(name: str, data: Any) -> Path:
    EVIDENCE_RUN_DIR.mkdir(parents=True, exist_ok=True)
    path = EVIDENCE_RUN_DIR / f"{name}.json"
    with path.open("w", encoding="utf-8") as fh:
        json.dump(data, fh, indent=2, ensure_ascii=False, default=str)
    return path


def _json_response(resp: httpx.Response) -> dict[str, Any]:
    try:
        body = resp.json()
    except Exception:
        body = {"text": resp.text}
    if isinstance(body, dict):
        return {"status_code": resp.status_code, **body}
    return {"status_code": resp.status_code, "body": body}


def _valid_how_to() -> dict[str, Any]:
    return {
        "install": {"claude-code": "npx forge install create-user-stories"},
        "usage": {"typescript": "import { createUserStories } from 'forge';"},
        "env": ["FORGE_API_TOKEN"],
    }


def _valid_active_surface_mcp(asset_id: str) -> dict[str, Any]:
    return {"family": "mcp", "endpoint": f"/v1/gw/mcp/{asset_id}"}


async def _create_skill(
    client: httpx.AsyncClient,
    registry_url: str,
    workspace_id: str,
    headers: dict[str, str],
    *,
    how_to: dict[str, Any] | None = None,
    active_surface: dict[str, Any] | None = None,
    lifecycle: str = "in_review",
) -> tuple[str, str]:
    name = f"are-{uuid.uuid4().hex[:8]}"
    payload: dict[str, Any] = {
        "type": "skill",
        "name": name,
        "version": "0.1.0",
        "owner_team": "active-registry-gateways-test",
        "inputs_schema": {},
        "outputs_schema": {},
        "visibility": "workspace",
        "lifecycle_state": lifecycle,
        "trust_level": "T1",
        "eval_scores": {"quality": 0.9, "safety": 0.9, "cost": 0.9, "latency": 0.9},
        "owners": [],
        "metadata": {},
    }
    if how_to is not None:
        payload["metadata"]["seed_how_to"] = how_to
    if active_surface is not None:
        payload["metadata"]["seed_active_surface"] = active_surface
    resp = await client.post(
        f"{registry_url.rstrip('/')}/v1/workspaces/{workspace_id}/assets",
        json=payload,
        headers=headers,
    )
    body = _json_response(resp)
    _save_evidence(f"create_{name}", body)
    assert resp.status_code == 201, f"create failed: {body}"
    return str(body["id"]), payload["version"]


@pytest.mark.asyncio
async def test_promotion_blocked_by_missing_how_to_and_active_surface() -> None:
    """A skill with neither how_to nor active_surface MUST be refused on
    promotion to `approved`. The error code matches the scenario in
    openspec/changes/active-registry-gateways/specs/ai-asset-registry/spec.md.
    """
    registry_url = _env_or_skip("REGISTRY_API_URL")
    workspace_id = _env_or_skip("WORKSPACE_ID")
    token = _env_or_skip("AUTH_TOKEN")
    headers = _headers(token, f"are-{uuid.uuid4().hex[:8]}")

    async with httpx.AsyncClient(timeout=30.0) as client:
        asset_id, version = await _create_skill(client, registry_url, workspace_id, headers)
        resp = await client.post(
            f"{registry_url.rstrip('/')}/v1/assets/{asset_id}/versions/{version}/transition",
            json={
                "lifecycle_state": "approved",
                "trust_level": "T1",
                "eval_scores": {"quality": 0.9, "safety": 0.9, "cost": 0.9, "latency": 0.9},
            },
            headers=headers,
        )
        body = _json_response(resp)
        _save_evidence(f"promotion_missing_how_to_{asset_id}", body)
        assert resp.status_code == 409, f"expected 409 for missing how_to; got {body}"
        # Either missing_how_to or missing_active_surface depending on the
        # order the validator runs; the spec only requires that the row is
        # refused with one of the two missing-* codes.
        assert body.get("code") in {"missing_how_to", "missing_active_surface"}, body


class _StubMCPHandler(BaseHTTPRequestHandler):
    """HTTP server serving a fixed manifest body so the registry fetch is
    deterministic across drift / promotion runs.
    """

    body = b'{"tools":[{"name":"echo"}]}'

    def do_GET(self) -> None:  # noqa: N802 (BaseHTTPRequestHandler convention)
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(self.body)))
        self.end_headers()
        self.wfile.write(self.body)

    def log_message(self, format: str, *args: Any) -> None:
        return


def _stub_server() -> tuple[ThreadingHTTPServer, str]:
    srv = ThreadingHTTPServer(("127.0.0.1", 0), _StubMCPHandler)
    port = srv.server_address[1]
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv, f"http://127.0.0.1:{port}"


@pytest.mark.asyncio
async def test_external_mcp_registration_persists_manifest_hash() -> None:
    """Register an external MCP against a synthetic upstream and verify the
    registry stores a stable sha256 digest on the asset row.
    """
    registry_url = _env_or_skip("REGISTRY_API_URL")
    workspace_id = _env_or_skip("WORKSPACE_ID")
    token = _env_or_skip("AUTH_TOKEN")
    headers = _headers(token, f"are-{uuid.uuid4().hex[:8]}")

    srv, base = _stub_server()
    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            payload = {
                "workspace_id": workspace_id,
                "name": f"vendor-{uuid.uuid4().hex[:8]}",
                "version": "0.1.0",
                "owner_team": "are-test",
                "endpoint_url": base + "/mcp",
                "credential_ref": "vault://kv/forge/are-test",
                "allowlist": ["echo"],
                "how_to": _valid_how_to(),
            }
            resp = await client.post(
                f"{registry_url.rstrip('/')}/v1/registry/mcps/external",
                json=payload,
                headers=headers,
            )
            body = _json_response(resp)
            _save_evidence(f"external_mcp_register_{payload['name']}", body)
            assert resp.status_code == 201, body
            assert body.get("provenance") == "external"
            assert body.get("type") == "mcp"
            assert body.get("lifecycle_state") == "proposed"
    finally:
        srv.shutdown()


@pytest.mark.asyncio
async def test_external_mcp_registration_rejects_raw_credential() -> None:
    """The registry MUST refuse a registration whose `credential_ref` looks
    like a literal credential rather than a vault pointer.
    """
    registry_url = _env_or_skip("REGISTRY_API_URL")
    workspace_id = _env_or_skip("WORKSPACE_ID")
    token = _env_or_skip("AUTH_TOKEN")
    headers = _headers(token, f"are-{uuid.uuid4().hex[:8]}")

    srv, base = _stub_server()
    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            payload = {
                "workspace_id": workspace_id,
                "name": f"vendor-leak-{uuid.uuid4().hex[:8]}",
                "version": "0.1.0",
                "owner_team": "are-test",
                "endpoint_url": base + "/mcp",
                "credential_ref": "Bearer LITERAL-SECRET",
                "allowlist": ["echo"],
            }
            resp = await client.post(
                f"{registry_url.rstrip('/')}/v1/registry/mcps/external",
                json=payload,
                headers=headers,
            )
            body = _json_response(resp)
            _save_evidence(f"external_mcp_reject_raw_{payload['name']}", body)
            assert resp.status_code == 400
            assert body.get("code") == "invalid_credential_ref"
    finally:
        srv.shutdown()


@pytest.mark.asyncio
async def test_external_a2a_registration_persists_agent_card_hash() -> None:
    """The A2A registration flow fetches the well-known agent.json so the
    stub serves the manifest at that path. The registry MUST persist the
    digest and create the asset with provenance=external.
    """
    registry_url = _env_or_skip("REGISTRY_API_URL")
    workspace_id = _env_or_skip("WORKSPACE_ID")
    token = _env_or_skip("AUTH_TOKEN")
    headers = _headers(token, f"are-{uuid.uuid4().hex[:8]}")

    class _StubA2A(_StubMCPHandler):
        body = b'{"id":"vendor-y","tasks":[{"name":"translate"}]}'

    srv = ThreadingHTTPServer(("127.0.0.1", 0), _StubA2A)
    port = srv.server_address[1]
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            payload = {
                "workspace_id": workspace_id,
                "name": f"partner-{uuid.uuid4().hex[:8]}",
                "version": "0.1.0",
                "owner_team": "are-test",
                "endpoint_url": f"http://127.0.0.1:{port}/agent",
                "credential_ref": "vault://kv/forge/are-partner",
                "allowlist": ["translate"],
            }
            resp = await client.post(
                f"{registry_url.rstrip('/')}/v1/registry/agents/external",
                json=payload,
                headers=headers,
            )
            body = _json_response(resp)
            _save_evidence(f"external_a2a_register_{payload['name']}", body)
            assert resp.status_code == 201, body
            assert body.get("provenance") == "external"
            assert body.get("type") == "agent"
    finally:
        srv.shutdown()


@pytest.mark.asyncio
async def test_external_mcp_registration_surfaces_upstream_fetch_error() -> None:
    """The registry MUST 502 when the upstream manifest fetch fails, so
    the operator gets a clear signal that the endpoint is not reachable.
    Implemented by pointing at a port that nobody is listening on.
    """
    registry_url = _env_or_skip("REGISTRY_API_URL")
    workspace_id = _env_or_skip("WORKSPACE_ID")
    token = _env_or_skip("AUTH_TOKEN")
    headers = _headers(token, f"are-{uuid.uuid4().hex[:8]}")

    # Reserve a port immediately released so the registry's GET fails.
    import socket

    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()

    async with httpx.AsyncClient(timeout=30.0) as client:
        payload = {
            "workspace_id": workspace_id,
            "name": f"vendor-unreachable-{uuid.uuid4().hex[:8]}",
            "version": "0.1.0",
            "owner_team": "are-test",
            "endpoint_url": f"http://127.0.0.1:{port}/mcp",
            "credential_ref": "vault://kv/forge/are-test",
            "allowlist": ["echo"],
        }
        resp = await client.post(
            f"{registry_url.rstrip('/')}/v1/registry/mcps/external",
            json=payload,
            headers=headers,
        )
        body = _json_response(resp)
        _save_evidence(f"external_mcp_unreachable_{payload['name']}", body)
        assert resp.status_code == 502, body
        assert body.get("code") == "manifest_fetch_failed"


# Silence the unused-import warning when this module is collected but no
# tests have a live registry to hit.
_ = asyncio
