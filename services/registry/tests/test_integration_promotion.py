from __future__ import annotations

import json
import os
import uuid
from datetime import datetime
from pathlib import Path

import pytest
import httpx


EVIDENCE_BASE = Path("docs/governance/evidence/phase-1")


def _env_or_skip(name: str) -> str:
    v = os.getenv(name)
    if not v:
        pytest.skip(f"integration test skipped: env var {name} is not set")
    return v


async def _create_asset(client: httpx.AsyncClient, registry_url: str, workspace_id: str, headers: dict[str, str], *,
                        name: str | None = None, version: str = "0.1.0", lifecycle: str = "proposed",
                        trust_level: str = "T1", eval_scores: dict | None = None) -> tuple[str, str]:
    if name is None:
        name = f"integration-{uuid.uuid4().hex[:8]}"
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
        "metadata": {},
    }
    url = f"{registry_url.rstrip('/')}/v1/workspaces/{workspace_id}/assets"
    r = await client.post(url, json=payload, headers=headers)
    text = r.text
    try:
        body = r.json() if r.status_code == 201 else {"status_code": r.status_code, "text": text}
    except Exception:
        body = {"status_code": r.status_code, "text": text}
    assert r.status_code == 201, f"create asset failed: {body}"
    asset_id = body.get("id")
    return asset_id, version


def _save_evidence(name: str, data: dict | list | str) -> None:
    ts = datetime.utcnow().strftime("%Y%m%dT%H%M%SZ")
    dest = EVIDENCE_BASE / ts
    dest.mkdir(parents=True, exist_ok=True)
    path = dest / f"{name}.json"
    with path.open("w", encoding="utf-8") as fh:
        json.dump(data, fh, indent=2, ensure_ascii=False)


@pytest.mark.asyncio
async def test_registry_promotion_and_invocation_blocking_integration() -> None:
    """Integration test for tasks 13.5 and 13.6.

    Requires the following environment variables to be set in the test runner:
      - REGISTRY_API_URL
      - WORKSPACE_ID
      - AUTH_TOKEN (Bearer token with rights to create/transition assets)

    The test will:
      - create an asset and attempt to promote it with failing evals (expect rejection)
      - promote it with passing evals (expect success)
      - create an `in_review` asset and assert invocation in `prod` is blocked
    """

    registry_url = _env_or_skip("REGISTRY_API_URL")
    workspace_id = _env_or_skip("WORKSPACE_ID")
    token = _env_or_skip("AUTH_TOKEN")
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

    async with httpx.AsyncClient(timeout=30.0) as client:
        # 1) Create asset and attempt failing promotion
        asset_name = f"promote-{uuid.uuid4().hex[:8]}"
        asset_id, version = await _create_asset(client, registry_url, workspace_id, headers, name=asset_name)

        failing_scores = {"quality": 0.7, "safety": 0.7, "cost": 0.7, "latency": 0.7}
        payload_fail = {"lifecycle_state": "approved", "trust_level": "T1", "eval_scores": failing_scores}
        url_transition = f"{registry_url.rstrip('/')}/v1/assets/{asset_id}/versions/{version}/transition"
        r_fail = await client.post(url_transition, json=payload_fail, headers=headers)
        # Save evidence
        try:
            _save_evidence(f"asset_{asset_name}_transition_fail", r_fail.json())
        except Exception:
            _save_evidence(f"asset_{asset_name}_transition_fail", {"status_code": r_fail.status_code, "text": r_fail.text})
        assert r_fail.status_code == 400, f"expected promotion to fail with 400 due to eval thresholds, got {r_fail.status_code}: {r_fail.text}"
        body_fail = r_fail.json()
        assert body_fail.get("code") == "eval_threshold_failed", f"unexpected failure payload: {body_fail}"

        # 2) Promote with passing evals
        passing_scores = {"quality": 0.85, "safety": 0.85, "cost": 0.85, "latency": 0.85}
        payload_ok = {"lifecycle_state": "approved", "trust_level": "T1", "eval_scores": passing_scores}
        r_ok = await client.post(url_transition, json=payload_ok, headers=headers)
        try:
            _save_evidence(f"asset_{asset_name}_transition_ok", r_ok.json())
        except Exception:
            _save_evidence(f"asset_{asset_name}_transition_ok", {"status_code": r_ok.status_code, "text": r_ok.text})
        assert r_ok.status_code == 200, f"promotion with passing evals failed: {r_ok.status_code} {r_ok.text}"
        body_ok = r_ok.json()
        assert body_ok.get("lifecycle_state") == "approved"

        # 3) Create an in_review asset and verify invocation check in prod is blocked
        inrev_name = f"inreview-{uuid.uuid4().hex[:8]}"
        asset2_id, version2 = await _create_asset(client, registry_url, workspace_id, headers, name=inrev_name, lifecycle="in_review")
        url_invoke_check = f"{registry_url.rstrip('/')}/v1/assets/{asset2_id}/versions/{version2}/invoke-check"
        r_invoke = await client.post(url_invoke_check, json={"environment": "prod"}, headers=headers)
        try:
            _save_evidence(f"asset_{inrev_name}_invoke_prod", r_invoke.json())
        except Exception:
            _save_evidence(f"asset_{inrev_name}_invoke_prod", {"status_code": r_invoke.status_code, "text": r_invoke.text})
        assert r_invoke.status_code == 200
        body_invoke = r_invoke.json()
        assert body_invoke.get("allowed") is False, f"expected prod invocation to be blocked for in_review asset: {body_invoke}"

        # also assert dev invocations are allowed
        r_invoke_dev = await client.post(url_invoke_check, json={"environment": "dev"}, headers=headers)
        try:
            _save_evidence(f"asset_{inrev_name}_invoke_dev", r_invoke_dev.json())
        except Exception:
            _save_evidence(f"asset_{inrev_name}_invoke_dev", {"status_code": r_invoke_dev.status_code, "text": r_invoke_dev.text})
        assert r_invoke_dev.status_code == 200
        assert r_invoke_dev.json().get("allowed") is True
