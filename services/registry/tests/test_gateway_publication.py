"""Integration tests for the gateway-publish lifecycle hook.

These tests run against a live `services/registry` plus the supporting Postgres
and Kafka stack from `deploy/compose`. They are skipped when the required env
vars are not present so the test file is safe to ship in CI without a backend.

Required env (when running for real):
  REGISTRY_URL                e.g. http://localhost:8082
  REGISTRY_TEST_TOKEN         a developer Bearer token accepted by the registry
  REGISTRY_TEST_WORKSPACE_ID  a uuid workspace the token can edit
"""

from __future__ import annotations

import os
import uuid

import httpx
import pytest


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


def _create_skill(client: httpx.Client, workspace_id: str, token: str) -> dict:
    """Create a fresh approved+T2 skill that is gateway-eligible."""
    name = f"gw-test-{uuid.uuid4().hex[:8]}"
    create = client.post(
        f"/v1/workspaces/{workspace_id}/assets",
        headers=_headers(token),
        json={
            "type": "skill",
            "name": name,
            "version": "1.0.0",
            "owner_team": "platform-engineering",
            "inputs_schema": {"type": "object"},
            "outputs_schema": {"type": "object"},
            "visibility": "workspace",
            "trust_level": "T2",
        },
    )
    create.raise_for_status()
    asset = create.json()
    # Drive lifecycle to approved (test helper assumes pipeline-green + owner-approve
    # are exercised elsewhere; here we use the simple transition endpoint).
    for next_state, trust in (("in_review", "T2"), ("approved", "T2")):
        resp = client.post(
            f"/v1/assets/{asset['id']}/versions/1.0.0/transition",
            headers=_headers(token),
            json={
                "lifecycle_state": next_state,
                "trust_level": trust,
                "eval_scores": {"quality": 0.95, "safety": 0.95, "cost": 0.95, "latency": 0.95},
            },
        )
        resp.raise_for_status()
    return resp.json()


@pytest.fixture(scope="module")
def client() -> httpx.Client:
    base = _env_or_skip("REGISTRY_URL")
    with httpx.Client(base_url=base, timeout=30.0) as c:
        yield c


@pytest.fixture(scope="module")
def workspace_id() -> str:
    return _env_or_skip("REGISTRY_TEST_WORKSPACE_ID")


@pytest.fixture(scope="module")
def token() -> str:
    return _env_or_skip("REGISTRY_TEST_TOKEN")


def test_publish_success(client: httpx.Client, workspace_id: str, token: str) -> None:
    asset = _create_skill(client, workspace_id, token)
    resp = client.post(
        f"/v1/assets/{asset['id']}/versions/1.0.0/lifecycle-hooks/gateway-publish",
        headers=_headers(token),
        json={
            "channel": "stable",
            "package_digest": "sha256:" + "0" * 64,
            "signature_id": "test-sig-1",
            "attestation_id": "test-att-1",
            "bytes_uri": "s3://forge-packages/test.tar.zst",
            "size_bytes": 1024,
        },
    )
    assert resp.status_code == 200, resp.text
    published = resp.json()
    assert published["distribution"]["gateway_published"] is True
    assert published["distribution"]["gateway_channel"] == "stable"
    assert published["distribution"]["package_digest"] == "sha256:" + "0" * 64


def test_publish_rejected_missing_signature(client: httpx.Client, workspace_id: str, token: str) -> None:
    asset = _create_skill(client, workspace_id, token)
    resp = client.post(
        f"/v1/assets/{asset['id']}/versions/1.0.0/lifecycle-hooks/gateway-publish",
        headers=_headers(token),
        json={"channel": "stable"},
    )
    assert resp.status_code == 400
    assert resp.json().get("code") == "signature_invalid"


def test_publish_rejected_when_not_approved(client: httpx.Client, workspace_id: str, token: str) -> None:
    """A skill in `proposed` state cannot be published."""
    name = f"gw-prop-{uuid.uuid4().hex[:8]}"
    create = client.post(
        f"/v1/workspaces/{workspace_id}/assets",
        headers=_headers(token),
        json={
            "type": "skill",
            "name": name,
            "version": "1.0.0",
            "owner_team": "platform-engineering",
            "inputs_schema": {"type": "object"},
            "outputs_schema": {"type": "object"},
            "trust_level": "T2",
        },
    )
    create.raise_for_status()
    asset = create.json()
    resp = client.post(
        f"/v1/assets/{asset['id']}/versions/1.0.0/lifecycle-hooks/gateway-publish",
        headers=_headers(token),
        json={
            "channel": "stable",
            "package_digest": "sha256:" + "0" * 64,
            "signature_id": "test-sig-2",
            "attestation_id": "test-att-2",
            "bytes_uri": "s3://forge-packages/test.tar.zst",
            "size_bytes": 1024,
        },
    )
    assert resp.status_code == 409
    assert resp.json().get("code") == "distribution_invariant_violated"


def test_unpublish_on_deprecate(client: httpx.Client, workspace_id: str, token: str) -> None:
    asset = _create_skill(client, workspace_id, token)
    pub = client.post(
        f"/v1/assets/{asset['id']}/versions/1.0.0/lifecycle-hooks/gateway-publish",
        headers=_headers(token),
        json={
            "channel": "stable",
            "package_digest": "sha256:" + "1" * 64,
            "signature_id": "test-sig-3",
            "attestation_id": "test-att-3",
            "bytes_uri": "s3://forge-packages/test.tar.zst",
            "size_bytes": 1024,
        },
    )
    pub.raise_for_status()
    deprecate = client.post(
        f"/v1/assets/{asset['id']}/versions/1.0.0/transition",
        headers=_headers(token),
        json={
            "lifecycle_state": "deprecated",
            "trust_level": "T2",
            "eval_scores": {"quality": 0.95, "safety": 0.95, "cost": 0.95, "latency": 0.95},
        },
    )
    deprecate.raise_for_status()
    body = deprecate.json()
    assert body["distribution"]["gateway_published"] is False


def test_publish_mcp_requires_remote_transport(client: httpx.Client, workspace_id: str, token: str) -> None:
    name = f"gw-mcp-{uuid.uuid4().hex[:8]}"
    create = client.post(
        f"/v1/workspaces/{workspace_id}/assets",
        headers=_headers(token),
        json={
            "type": "mcp",
            "name": name,
            "version": "1.0.0",
            "owner_team": "platform-engineering",
            "inputs_schema": {"type": "object"},
            "outputs_schema": {"type": "object"},
            "trust_level": "T2",
        },
    )
    create.raise_for_status()
    asset = create.json()
    for next_state in ("in_review", "approved"):
        client.post(
            f"/v1/assets/{asset['id']}/versions/1.0.0/transition",
            headers=_headers(token),
            json={
                "lifecycle_state": next_state,
                "trust_level": "T2",
                "eval_scores": {"quality": 0.95, "safety": 0.95, "cost": 0.95, "latency": 0.95},
            },
        ).raise_for_status()
    resp = client.post(
        f"/v1/assets/{asset['id']}/versions/1.0.0/lifecycle-hooks/gateway-publish",
        headers=_headers(token),
        json={"channel": "stable"},
    )
    assert resp.status_code == 409
    assert resp.json().get("code") == "remote_transport_required"
