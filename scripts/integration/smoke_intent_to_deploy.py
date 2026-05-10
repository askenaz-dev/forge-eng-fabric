#!/usr/bin/env python3
"""Integration smoke test for the reference workflow.

Triggers `forge.reference.intent-to-deploy@1` against the local stack (or
ephemeral CI infrastructure when `FORGE_SMOKE_BASE_URLS` points at one) and
asserts the presence and ordering of milestone events:

    intent.committed.v1
    repo.scaffolded.v1
    pr.opened.v1
    ci.passed.v1
    approval.granted.v1
    deploy.completed.v1

All milestones MUST share the same `correlation_id`. Exits non-zero on the
first missing or out-of-order milestone, naming the milestone in the failure
message so CI surfaces it directly.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent.parent
DEMO = REPO / "scripts" / "demo_intent_to_deploy.py"

EXPECTED_MILESTONES = [
    "intent.committed.v1",
    "repo.scaffolded.v1",
    "pr.opened.v1",
    "ci.passed.v1",
    "approval.granted.v1",
    "deploy.completed.v1",
]


def main() -> int:
    if not DEMO.exists():
        print(f"FAIL: demo script not found at {DEMO}", file=sys.stderr)
        return 2

    env = os.environ.copy()
    env.setdefault("WORKSPACE_ID", "22222222-2222-4222-8222-222222222222")
    env.setdefault("RUNTIME_ID", "rt-smoke-1")

    print("==> running make demo-intent-to-deploy")
    result = subprocess.run(
        [sys.executable, str(DEMO)],
        env=env,
        capture_output=True,
        text=True,
    )
    print(result.stdout)
    if result.returncode != 0:
        print(result.stderr, file=sys.stderr)
        print(f"FAIL: demo script exited {result.returncode}", file=sys.stderr)
        return result.returncode

    # Locate the latest report file
    report_dir = REPO / "build" / "demo-intent-to-deploy"
    reports = sorted(report_dir.glob("*.json"))
    if not reports:
        print(f"FAIL: no report found under {report_dir}", file=sys.stderr)
        return 1
    report = json.loads(reports[-1].read_text(encoding="utf-8"))

    if report.get("status") != "ok":
        print(f"FAIL: report status is {report.get('status')!r}", file=sys.stderr)
        return 1

    correlation_id = report.get("correlation_id")
    if not correlation_id:
        print("FAIL: report missing correlation_id", file=sys.stderr)
        return 1

    # Find the workflow.trigger step's milestones
    workflow_step = next((s for s in report.get("steps", []) if s.get("step") == "workflow.trigger"), None)
    if workflow_step is None or workflow_step.get("status") != "ok":
        print(f"FAIL: workflow.trigger step missing or failed", file=sys.stderr)
        return 1
    milestones = (workflow_step.get("result") or {}).get("milestones") or []

    # Assert order and presence
    actual_events = [m.get("event") for m in milestones]
    print(f"==> asserting milestone order: {actual_events}")

    actual_iter = iter(actual_events)
    for expected in EXPECTED_MILESTONES:
        for actual in actual_iter:
            if actual == expected:
                break
        else:
            print(f"FAIL: missing milestone {expected!r}", file=sys.stderr)
            print(f"      seen: {actual_events}", file=sys.stderr)
            return 1

    # Assert correlation_id consistency
    for m in milestones:
        if m.get("correlation_id") != correlation_id:
            print(
                f"FAIL: milestone {m.get('event')!r} has correlation_id "
                f"{m.get('correlation_id')!r} != {correlation_id!r}",
                file=sys.stderr,
            )
            return 1

    print(f"OK: all {len(EXPECTED_MILESTONES)} milestones present in order with consistent correlation_id")
    return 0


if __name__ == "__main__":
    sys.exit(main())
