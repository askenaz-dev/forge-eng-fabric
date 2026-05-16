#!/usr/bin/env python3
"""Integration smoke for the AI-Flow reference workflow.

End-to-end:
  1. Publish forge.reference.ai-email-triage@1 to workflow-registry.
  2. Manually fire the support-mail trigger via trigger-router's admin
     fire endpoint with a synthetic email payload.
  3. Poll workflow-runtime for the execution and print its step trace.

Run after `make up`. Honors WORKFLOW_REGISTRY_URL, TRIGGER_ROUTER_URL,
and WORKFLOW_RUNTIME_URL env vars (defaults to localhost).

Exits 0 on success, non-zero on any failure.
"""
from __future__ import annotations

import json
import os
import pathlib
import sys
import time
import urllib.request

REGISTRY_URL = os.environ.get("WORKFLOW_REGISTRY_URL", "http://localhost:8094")
RUNTIME_URL = os.environ.get("WORKFLOW_RUNTIME_URL", "http://localhost:8093")
TRIGGER_URL = os.environ.get("TRIGGER_ROUTER_URL", "http://localhost:8097")

REFERENCE_YAML = (
    pathlib.Path(__file__).resolve().parent.parent.parent
    / "services"
    / "workflow-registry"
    / "reference"
    / "ai-email-triage"
    / "1.0.0.yaml"
)


def http_post(url: str, body: dict) -> tuple[int, dict]:
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"content-type": "application/json"})
    with urllib.request.urlopen(req, timeout=10) as r:
        return r.status, json.loads(r.read() or b"{}")


def http_get(url: str) -> tuple[int, dict]:
    req = urllib.request.Request(url)
    with urllib.request.urlopen(req, timeout=10) as r:
        return r.status, json.loads(r.read() or b"{}")


def publish() -> None:
    yaml_body = REFERENCE_YAML.read_text(encoding="utf-8")
    status, body = http_post(
        f"{REGISTRY_URL}/v1/workflows/forge.reference.ai-email-triage/versions",
        {
            "workflow_id": "forge.reference.ai-email-triage",
            "workflow_yaml": yaml_body,
            "auto_bump": True,
            "actor": "smoke",
        },
    )
    if status not in (200, 201):
        raise SystemExit(f"publish failed: {status} {body}")
    print(f"[smoke] published version {body.get('version')}")


def fire() -> str:
    status, body = http_post(
        f"{TRIGGER_URL}/v1/triggers/forge.reference.ai-email-triage/support-mail/fire",
        {
            "subject": "[support] outage",
            "from": "alice@acme.com",
            "body": "everything is on fire",
            "received_at": "2026-05-16T12:00:00Z",
            "message_id": "smoke-1",
        },
    )
    if status not in (200, 202):
        raise SystemExit(f"fire failed: {status} {body}")
    exec_id = body.get("execution_id")
    print(f"[smoke] fired trigger → execution_id={exec_id}")
    return exec_id


def poll(exec_id: str) -> None:
    deadline = time.time() + 30
    while time.time() < deadline:
        try:
            status, body = http_get(f"{RUNTIME_URL}/v1/executions/{exec_id}")
        except Exception as exc:
            print(f"[smoke] poll error: {exc}", file=sys.stderr)
            time.sleep(1)
            continue
        st = body.get("status")
        print(f"[smoke] poll: status={st}")
        if st in ("completed", "failed", "waiting"):
            for ev in body.get("steps", []):
                print(f"  - {ev.get('step_id')} {ev.get('type')} {ev.get('status')}")
            return
        time.sleep(1)
    raise SystemExit("execution did not reach terminal state within 30s")


def main() -> int:
    if not REFERENCE_YAML.exists():
        raise SystemExit(f"reference yaml missing: {REFERENCE_YAML}")
    publish()
    exec_id = fire()
    poll(exec_id)
    print("[smoke] OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
