#!/usr/bin/env python3
"""make demo-intent-to-infrastructure — drive forge.reference.intent-to-infrastructure@1 end-to-end.

Steps:
  1. Submit a canned intent through Alfred (POST /v1/intent/start).
  2. Commit the intent to obtain an openspec_id.
  3. Start the workflow forge.reference.intent-to-infrastructure@1 with a fully-required
     targets matrix and include=[iac, observability] opt-ins.
  4. Poll the workflow run until completion, auto-approving HITL gates via the
     documented test-only fixture (X-Forge-Demo-Auto-Approve: true).
  5. Write a JSON report at build/demo-intent-to-infrastructure/<timestamp>.json
     summarising each step, the deploy URL and observability dashboard URLs.

Exits 0 on success, non-zero on any step failure.
"""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
import uuid
from datetime import UTC, datetime
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
REPORT_DIR = REPO / "build" / "demo-intent-to-infrastructure"

DEFAULT_ALFRED = os.environ.get("ALFRED_URL", "http://localhost:8090")
DEFAULT_OPENSPEC = os.environ.get("OPENSPEC_URL", "http://localhost:8083")
DEFAULT_WORKFLOW_RUNTIME = os.environ.get("WORKFLOW_RUNTIME_URL", "http://localhost:8095")
DEFAULT_APPROVALS = os.environ.get("APPROVALS_URL", "http://localhost:8105")
DEFAULT_APPLICATION = os.environ.get("APPLICATION_URL", "http://localhost:8095")

WORKFLOW_ID = "forge.reference.intent-to-infrastructure@1"
CANNED_INTENT = {
    "title": "Demo: Intent-to-Infrastructure smoke test",
    "summary": "Canned intent exercising the full SDLC chain from architecture to IaC.",
    "problem": "Platform smoke test for the intent-to-infrastructure reference workflow.",
    "solution": "Run the full SDLC workflow with all targets set to required and iac/observability opted in.",
}


def http(url: str, *, method: str = "GET", body: dict | None = None, extra_headers: dict | None = None, timeout: int = 60) -> tuple[int, dict]:
    data = json.dumps(body).encode("utf-8") if body is not None else None
    headers = {"content-type": "application/json"}
    if extra_headers:
        headers.update(extra_headers)
    req = urllib.request.Request(url, method=method, data=data, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            payload = resp.read().decode("utf-8")
            return resp.status, json.loads(payload) if payload else {}
    except urllib.error.HTTPError as exc:
        body_text = exc.read().decode("utf-8", errors="replace")
        return exc.code, {"error": body_text}


def step(label: str, *, success: bool = True, data: dict | None = None) -> dict:
    result = {
        "step": label,
        "at": datetime.now(UTC).isoformat(),
        "outcome": "ok" if success else "fail",
    }
    if data:
        result.update(data)
    print(f"  {'✓' if success else '✗'} {label}", flush=True)
    return result


def main() -> int:
    correlation_id = str(uuid.uuid4())
    report: list[dict] = []
    print(f"\n==> demo-intent-to-infrastructure  correlation_id={correlation_id}\n")

    alfred = os.environ.get("ALFRED_URL", DEFAULT_ALFRED)
    workflow_runtime = os.environ.get("WORKFLOW_RUNTIME_URL", DEFAULT_WORKFLOW_RUNTIME)
    approvals = os.environ.get("APPROVALS_URL", DEFAULT_APPROVALS)

    # Step 1: Start intent
    status, body = http(f"{alfred}/v1/intent/start", method="POST", body={**CANNED_INTENT, "correlation_id": correlation_id})
    if status not in (200, 201):
        report.append(step("start-intent", success=False, data={"status": status, "body": body}))
        return _write_report(report, correlation_id, success=False)
    intent_id = body.get("intent_id") or body.get("id", "unknown")
    report.append(step("start-intent", data={"intent_id": intent_id}))

    # Step 2: Commit intent
    status, body = http(f"{alfred}/v1/intent/{intent_id}/commit", method="POST", body={"correlation_id": correlation_id})
    if status not in (200, 201):
        report.append(step("commit-intent", success=False, data={"status": status, "body": body}))
        return _write_report(report, correlation_id, success=False)
    openspec_id = body.get("openspec_id", "unknown")
    app_id = body.get("app_id", "unknown")
    report.append(step("commit-intent", data={"openspec_id": openspec_id, "app_id": app_id}))

    # Step 3: Start the workflow
    run_payload = {
        "workflow_id": WORKFLOW_ID,
        "inputs": {
            "app_id": app_id,
            "openspec_id": openspec_id,
            "correlation_id": correlation_id,
            "include": ["iac", "observability"],
        },
        "targets_override": {
            "architect": "required",
            "design": "required",
            "development": "required",
            "qa": "required",
            "security": "required",
            "devops": "required",
            "iac": "required",
            "sre": "required",
            "observability": "required",
        },
    }
    status, body = http(f"{workflow_runtime}/v1/runs", method="POST", body=run_payload, extra_headers={"x-correlation-id": correlation_id})
    if status not in (200, 201, 202):
        report.append(step("start-workflow-run", success=False, data={"status": status, "body": body}))
        return _write_report(report, correlation_id, success=False)
    run_id = body.get("run_id") or body.get("id", "unknown")
    report.append(step("start-workflow-run", data={"run_id": run_id}))

    # Step 4: Poll and auto-approve HITL gates
    deploy_url: str | None = None
    dashboard_urls: list[str] = []
    demo_headers = {"x-forge-demo-auto-approve": "true", "x-correlation-id": correlation_id}
    max_polls = 120
    for _ in range(max_polls):
        time.sleep(5)
        status, body = http(f"{workflow_runtime}/v1/runs/{run_id}", extra_headers={"x-correlation-id": correlation_id})
        run_status = body.get("status", "unknown")

        # Auto-approve pending HITL gates
        pending = body.get("pending_approvals", [])
        for gate in pending:
            gate_id = gate.get("id", "unknown")
            http(f"{approvals}/v1/approvals/{gate_id}/approve", method="POST",
                 body={"approved_by": "demo-auto-approve", "reason": "demo smoke test"},
                 extra_headers=demo_headers)
            report.append(step(f"auto-approve-gate:{gate_id}"))

        if run_status == "completed":
            deploy_url = body.get("outputs", {}).get("deploy_url")
            dashboard_urls = body.get("outputs", {}).get("observability_urls", [])
            report.append(step("workflow-completed", data={"deploy_url": deploy_url, "dashboard_urls": dashboard_urls}))
            break
        if run_status in ("failed", "cancelled"):
            report.append(step("workflow-completed", success=False, data={"run_status": run_status, "body": body}))
            return _write_report(report, correlation_id, success=False)
    else:
        report.append(step("workflow-timeout", success=False))
        return _write_report(report, correlation_id, success=False)

    print(f"\n  deploy_url       : {deploy_url}")
    print(f"  dashboard_urls   : {dashboard_urls}")
    return _write_report(report, correlation_id, success=True, deploy_url=deploy_url, dashboard_urls=dashboard_urls)


def _write_report(report: list[dict], correlation_id: str, *, success: bool, deploy_url: str | None = None, dashboard_urls: list | None = None) -> int:
    REPORT_DIR.mkdir(parents=True, exist_ok=True)
    ts = datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ")
    out = {
        "workflow": WORKFLOW_ID,
        "correlation_id": correlation_id,
        "generated_at": datetime.now(UTC).isoformat(),
        "success": success,
        "steps": report,
        "deploy_url": deploy_url,
        "observability_urls": dashboard_urls or [],
    }
    path = REPORT_DIR / f"{ts}.json"
    path.write_text(json.dumps(out, indent=2))
    print(f"\n  Report: {path}")
    return 0 if success else 1


if __name__ == "__main__":
    sys.exit(main())
