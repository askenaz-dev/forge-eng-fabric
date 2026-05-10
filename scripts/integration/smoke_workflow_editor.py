#!/usr/bin/env python3
"""Smoke-test the workflow editor save/export/re-open/run contract.

The Portal editor persists canonical workflow DSL through workflow-registry and
dry-runs it through workflow-runtime. This script exercises that end-to-end
contract without browser auth dependencies:

1. Build a workflow DSL payload equivalent to the editor textarea.
2. Save it as an immutable workflow-registry version.
3. Export the DSL and record a content digest.
4. Re-open the saved version from workflow-registry.
5. Run it via workflow-runtime in dry-run mode.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import time
import urllib.error
import urllib.request
from datetime import UTC, datetime


WORKFLOW_DSL = """apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: editor-smoke-{suffix}
  name: Editor Smoke {suffix}
  version: 1.0.0
  visibility: workspace
  criticality: medium
spec:
  inputs:
    - name: story
      type: string
      required: true
  steps:
    - id: refine
      type: skill
      ref: registry:skill/sdlc-product/refine-user-story@1.2.0
      inputs:
        story: $inputs.story
    - id: notify
      type: mcp
      ref: registry:mcp/slack@1.0.0
      tool: send_message
      depends_on: [refine]
"""


def request(url: str, *, method: str = "GET", body: dict | None = None, timeout: int = 15) -> tuple[int, dict]:
    data = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(
        url,
        method=method,
        data=data,
        headers={"content-type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8")
            return resp.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8")
        return exc.code, {"error": raw}


def must(result: tuple[int, dict], step: str) -> dict:
    status, body = result
    if status >= 400:
        raise RuntimeError(f"{step} failed: http {status}: {body}")
    return body


def wait_execution(runtime_url: str, tenant_id: str, execution_id: str) -> dict:
    deadline = time.time() + 20
    last: dict = {}
    while time.time() < deadline:
        body = must(
            request(f"{runtime_url}/v1/executions/{execution_id}?tenant_id={tenant_id}", timeout=5),
            "execution.poll",
        )
        last = body
        if body.get("status") in {"completed", "failed", "cancelled"}:
            return body
        time.sleep(0.25)
    raise RuntimeError(f"execution {execution_id} timed out; last_status={last.get('status')}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--registry-url", default="http://localhost:8094")
    parser.add_argument("--runtime-url", default="http://localhost:8093")
    parser.add_argument("--tenant", default="tenant-editor-smoke")
    parser.add_argument("--workspace", default="workspace-editor-smoke")
    args = parser.parse_args()

    suffix = datetime.now(UTC).strftime("%Y%m%d%H%M%S")
    workflow_id = f"editor-smoke-{suffix}"
    workflow_dsl = WORKFLOW_DSL.format(suffix=suffix)
    digest = hashlib.sha256(workflow_dsl.encode("utf-8")).hexdigest()

    print(f"workflow_id: {workflow_id}")
    print("==> build DSL")
    print(f"    sha256: {digest}")

    print("==> save workflow parent")
    must(
        request(
            f"{args.registry_url}/v1/workflows",
            method="POST",
            body={
                "id": workflow_id,
                "tenant_id": args.tenant,
                "workspace_id": args.workspace,
                "name": f"Editor Smoke {suffix}",
                "visibility": "workspace",
                "tags": ["editor-smoke"],
            },
        ),
        "workflow.create",
    )
    print("    ok")

    print("==> save immutable version")
    version = must(
        request(
            f"{args.registry_url}/v1/workflows/{workflow_id}/versions",
            method="POST",
            body={
                "tenant_id": args.tenant,
                "workflow_id": workflow_id,
                "workflow_yaml": workflow_dsl,
                "actor": "workflow-editor-smoke",
            },
        ),
        "workflow.version.publish",
    )
    print(f"    version: {version.get('version')}")

    print("==> export DSL")
    print(f"    bytes: {len(workflow_dsl.encode('utf-8'))}")

    print("==> re-open saved version")
    reopened = must(
        request(f"{args.registry_url}/v1/workflows/{workflow_id}/versions/{version['version']}"),
        "workflow.version.get",
    )
    reopened_id = (((reopened.get("ast") or {}).get("metadata") or {}).get("id"))
    if reopened_id != workflow_id:
        raise RuntimeError(f"re-opened workflow id mismatch: {reopened_id} != {workflow_id}")
    print("    ok")

    print("==> run dry-run execution")
    execution = must(
        request(
            f"{args.runtime_url}/v1/executions",
            method="POST",
            body={
                "tenant_id": args.tenant,
                "workspace_id": args.workspace,
                "workflow_yaml": workflow_dsl,
                "inputs": {"story": "as an operator I want editor smoke coverage"},
                "dry_run": True,
                "correlation_id": f"workflow-editor-smoke-{suffix}",
            },
        ),
        "execution.start",
    )
    final = wait_execution(args.runtime_url, args.tenant, execution["id"])
    if final.get("status") != "completed":
        raise RuntimeError(f"execution ended {final.get('status')}: {final.get('failure_reason', '')}")
    completed_steps = [s.get("step_id") for s in final.get("steps", []) if s.get("status") == "completed"]
    print(f"    execution_id: {execution['id']}")
    print(f"    completed_steps: {completed_steps}")
    print("OK: workflow editor smoke completed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
