from __future__ import annotations

import asyncio
import json
import os
import time
import uuid
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import httpx
import pytest

EVIDENCE_BASE = Path("docs/governance/evidence/phase-1")
EVIDENCE_RUN_DIR = EVIDENCE_BASE / datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ")


def _env_or_skip(name: str) -> str:
    value = os.getenv(name)
    if not value:
        pytest.skip(f"integration test skipped: env var {name} is not set")
    return value


def _token_or_skip(*names: str) -> str:
    for name in names:
        value = os.getenv(name)
        if value:
            return value
    joined = ", ".join(names)
    pytest.skip(f"integration test skipped: one of {joined} must be set")


def _headers(token: str, correlation_id: str | None = None) -> dict[str, str]:
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    if correlation_id:
        headers["X-Correlation-Id"] = correlation_id
    return headers


def _json_response(response: httpx.Response) -> dict[str, Any]:
    try:
        body = response.json()
    except Exception:
        body = {"text": response.text}
    if isinstance(body, dict):
        return {"status_code": response.status_code, **body}
    return {"status_code": response.status_code, "body": body}


def _save_evidence(name: str, data: dict[str, Any] | list[Any] | str) -> Path:
    EVIDENCE_RUN_DIR.mkdir(parents=True, exist_ok=True)
    path = EVIDENCE_RUN_DIR / f"{name}.json"
    with path.open("w", encoding="utf-8") as fh:
        json.dump(data, fh, indent=2, ensure_ascii=False, default=str)
    return path


async def _create_asset(
    client: httpx.AsyncClient,
    registry_url: str,
    workspace_id: str,
    headers: dict[str, str],
    *,
    name: str | None = None,
    version: str = "0.1.0",
    lifecycle: str = "proposed",
    trust_level: str = "T1",
    eval_scores: dict[str, Any] | None = None,
) -> tuple[str, str]:
    name = name or f"integration-{uuid.uuid4().hex[:8]}"
    payload = {
        "type": "skill",
        "name": name,
        "version": version,
        "owner_team": "integration-test",
        "inputs_schema": {},
        "outputs_schema": {},
        "visibility": "workspace",
        "lifecycle_state": lifecycle,
        "trust_level": trust_level,
        "eval_scores": eval_scores or {},
        "owners": [],
        "metadata": {"created_by": "phase-1-integration-test"},
    }
    response = await client.post(
        f"{registry_url.rstrip('/')}/v1/workspaces/{workspace_id}/assets",
        json=payload,
        headers=headers,
    )
    body = _json_response(response)
    _save_evidence(f"asset_{name}_create", body)
    assert response.status_code == 201, f"create asset failed: {body}"
    return str(body["id"]), version


async def _transition_asset(
    client: httpx.AsyncClient,
    registry_url: str,
    asset_id: str,
    version: str,
    headers: dict[str, str],
    payload: dict[str, Any],
) -> httpx.Response:
    return await client.post(
        f"{registry_url.rstrip('/')}/v1/assets/{asset_id}/versions/{version}/transition",
        json=payload,
        headers=headers,
    )


def _expected_tool_ids() -> set[str]:
    raw = os.getenv(
        "ALFRED_EXPECTED_TOOL_IDS",
        "mcp:openspec.create,skill:create-user-stories,skill:generate-test-cases",
    )
    return {item.strip() for item in raw.split(",") if item.strip()}


def _contains_value(value: Any, expected: str) -> bool:
    if isinstance(value, str):
        return expected in value
    if isinstance(value, dict):
        return any(_contains_value(item, expected) for item in value.values())
    if isinstance(value, list):
        return any(_contains_value(item, expected) for item in value)
    return False


async def _poll_alfred_decisions(
    client: httpx.AsyncClient,
    alfred_url: str,
    headers: dict[str, str],
    workspace_id: str,
    correlation_id: str,
) -> list[dict[str, Any]]:
    deadline = time.monotonic() + int(os.getenv("ALFRED_DECISION_POLL_SECONDS", "120"))
    last_body: dict[str, Any] | None = None

    while time.monotonic() < deadline:
        response = await client.get(
            f"{alfred_url.rstrip('/')}/v1/decisions",
            params={"workspace_id": workspace_id, "correlation_id": correlation_id},
            headers=headers,
        )
        last_body = _json_response(response)
        if response.status_code == 200:
            decisions = last_body.get("decisions")
            if isinstance(decisions, list) and decisions:
                return decisions
        await asyncio.sleep(3)

    _save_evidence(f"alfred_decisions_timeout_{correlation_id}", last_body or {})
    pytest.fail(f"No Alfred decisions found for correlation_id {correlation_id}")


def _langfuse_auth() -> tuple[dict[str, str], tuple[str, str] | None]:
    headers = {"Content-Type": "application/json"}
    public_key = os.getenv("LANGFUSE_PUBLIC_KEY")
    secret_key = os.getenv("LANGFUSE_SECRET_KEY")
    if public_key and secret_key:
        return headers, (public_key, secret_key)

    api_key = os.getenv("LANGFUSE_API_KEY")
    if not api_key:
        pytest.skip(
            "integration test skipped: LANGFUSE_PUBLIC_KEY/LANGFUSE_SECRET_KEY "
            "or LANGFUSE_API_KEY must be set"
        )
    headers["Authorization"] = api_key if " " in api_key else f"Bearer {api_key}"
    return headers, None


async def _poll_langfuse_trace(
    client: httpx.AsyncClient,
    langfuse_url: str,
    correlation_id: str,
) -> dict[str, Any]:
    headers, auth = _langfuse_auth()
    candidates = [
        (f"{langfuse_url.rstrip('/')}/api/public/traces/{correlation_id}", {}),
        (f"{langfuse_url.rstrip('/')}/api/public/traces", {"traceId": correlation_id}),
        (f"{langfuse_url.rstrip('/')}/api/public/traces", {"limit": "50"}),
    ]
    deadline = time.monotonic() + int(os.getenv("LANGFUSE_POLL_SECONDS", "120"))
    last_response: dict[str, Any] = {}

    while time.monotonic() < deadline:
        for url, params in candidates:
            response = await client.get(url, params=params, headers=headers, auth=auth)
            body = _json_response(response)
            last_response = {"endpoint": str(response.url), "response": body}
            if response.status_code == 200 and _contains_value(body, correlation_id):
                return last_response
        await asyncio.sleep(5)

    _save_evidence(f"langfuse_trace_timeout_{correlation_id}", last_response)
    pytest.fail(f"Langfuse trace not found for correlation_id {correlation_id}")


@pytest.mark.asyncio
async def test_registry_promotion_and_invocation_blocking_integration() -> None:
    """Validate Phase 1 tasks 13.5 and 13.6 against a deployed Registry."""

    registry_url = _env_or_skip("REGISTRY_API_URL")
    workspace_id = _env_or_skip("WORKSPACE_ID")
    token = _env_or_skip("AUTH_TOKEN")
    correlation_id = f"phase1-registry-{uuid.uuid4().hex[:8]}"
    headers = _headers(token, correlation_id)

    async with httpx.AsyncClient(timeout=30.0) as client:
        asset_name = f"promote-{uuid.uuid4().hex[:8]}"
        asset_id, version = await _create_asset(
            client,
            registry_url,
            workspace_id,
            headers,
            name=asset_name,
            lifecycle="in_review",
            trust_level="T1",
        )

        failing_scores = {"quality": 0.7, "safety": 0.7, "cost": 0.7, "latency": 0.7}
        response_fail = await _transition_asset(
            client,
            registry_url,
            asset_id,
            version,
            headers,
            {"lifecycle_state": "approved", "trust_level": "T1", "eval_scores": failing_scores},
        )
        body_fail = _json_response(response_fail)
        _save_evidence(f"asset_{asset_name}_transition_fail", body_fail)
        assert response_fail.status_code == 400, f"expected eval rejection: {body_fail}"
        assert body_fail.get("code") == "eval_threshold_failed"

        passing_scores = {"quality": 0.85, "safety": 0.85, "cost": 0.85, "latency": 0.85}
        response_ok = await _transition_asset(
            client,
            registry_url,
            asset_id,
            version,
            headers,
            {"lifecycle_state": "approved", "trust_level": "T1", "eval_scores": passing_scores},
        )
        body_ok = _json_response(response_ok)
        _save_evidence(f"asset_{asset_name}_transition_ok", body_ok)
        assert response_ok.status_code == 200, f"promotion with passing evals failed: {body_ok}"
        assert body_ok.get("lifecycle_state") == "approved"

        in_review_name = f"inreview-{uuid.uuid4().hex[:8]}"
        in_review_id, in_review_version = await _create_asset(
            client,
            registry_url,
            workspace_id,
            headers,
            name=in_review_name,
            lifecycle="in_review",
            trust_level="T1",
        )
        invoke_url = (
            f"{registry_url.rstrip('/')}/v1/assets/{in_review_id}"
            f"/versions/{in_review_version}/invoke-check"
        )

        response_prod = await client.post(invoke_url, json={"environment": "prod"}, headers=headers)
        body_prod = _json_response(response_prod)
        _save_evidence(f"asset_{in_review_name}_invoke_prod_blocked", body_prod)
        assert response_prod.status_code == 200
        assert body_prod.get("allowed") is False
        assert body_prod.get("audit_event_type") == "com.forge.asset.invocation.checked.v1"
        assert body_prod.get("correlation_id") == correlation_id

        response_dev = await client.post(invoke_url, json={"environment": "dev"}, headers=headers)
        body_dev = _json_response(response_dev)
        _save_evidence(f"asset_{in_review_name}_invoke_dev_allowed", body_dev)
        assert response_dev.status_code == 200
        assert body_dev.get("allowed") is True


@pytest.mark.asyncio
async def test_alfred_langfuse_full_e2e_evidence() -> None:
    """Exercise Alfred and verify Langfuse contains the same correlation id."""

    alfred_url = _env_or_skip("ALFRED_API_URL")
    workspace_id = _env_or_skip("WORKSPACE_ID")
    alfred_token = _token_or_skip("ALFRED_TOKEN", "AUTH_TOKEN")
    langfuse_url = os.getenv("LANGFUSE_API_URL") or os.getenv("LANGFUSE_HOST")
    if not langfuse_url:
        pytest.skip("integration test skipped: LANGFUSE_API_URL or LANGFUSE_HOST is not set")

    correlation_id = f"phase1-alfred-{uuid.uuid4().hex[:8]}"
    headers = _headers(alfred_token, correlation_id)
    intent_text = os.getenv(
        "ALFRED_E2E_INTENT_TEXT",
        "Create an OpenSpec and invoke create-user-stories and generate-test-cases "
        "for a Phase 1 integration evidence run.",
    )
    payload = {
        "workspace_id": workspace_id,
        "text": intent_text,
        "correlation_id": correlation_id,
        "openspec_id": f"phase1-int-{uuid.uuid4().hex[:8]}",
        "metadata": {"env": "dev", "evidence_run": EVIDENCE_RUN_DIR.name},
    }

    async with httpx.AsyncClient(timeout=30.0) as client:
        response = await client.post(
            f"{alfred_url.rstrip('/')}/v1/intents",
            json=payload,
            headers=headers,
        )
        body = _json_response(response)
        _save_evidence(f"alfred_intent_{correlation_id}", body)
        assert response.status_code in (200, 201), f"Alfred intent submit failed: {body}"

        decisions = await _poll_alfred_decisions(
            client,
            alfred_url,
            headers,
            workspace_id,
            correlation_id,
        )
        _save_evidence(f"alfred_decisions_{correlation_id}", decisions)

        assert all(decision.get("correlation_id") == correlation_id for decision in decisions)
        tool_ids = {
            str(decision.get("tool_id"))
            for decision in decisions
            if decision.get("tool_id")
        }
        expected_tool_ids = _expected_tool_ids()
        assert tool_ids & expected_tool_ids, (
            f"expected at least one of {sorted(expected_tool_ids)} in Alfred decisions; "
            f"got {sorted(tool_ids)}"
        )

        langfuse_trace = await _poll_langfuse_trace(client, langfuse_url, correlation_id)
        _save_evidence(f"langfuse_trace_{correlation_id}", langfuse_trace)
        assert _contains_value(langfuse_trace, correlation_id)
        assert any(_contains_value(langfuse_trace, tool_id) for tool_id in tool_ids)
