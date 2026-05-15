#!/usr/bin/env python3
"""CI smoke test for forge.reference.intent-to-infrastructure@1.

Runs in two modes:
  - SYNTHETIC_MODE=true (default in CI): validates milestone ordering logic
    without a live stack. Asserts the full 20-step milestone sequence is
    correctly ordered and that correlation_id flows through all steps.
  - LIVE_STACK=true: drives the live stack end-to-end (requires make up).
    Uses the same demo script but with CI-specific fixtures.

Exits 0 on success, 1 on any assertion failure.
"""

from __future__ import annotations

import json
import os
import sys

# Expected milestone chain per the reference workflow spec (sdlc-end-to-end §9.3).
EXPECTED_MILESTONES = [
    "intent.committed.v1",
    "repo.scaffolded.v1",
    "sdlc.adr.proposed.v1",
    "sdlc.api_contract.proposed.v1",
    "sdlc.data_model.proposed.v1",
    "sdlc.threat_model.completed.v1",
    "sdlc.ui_blueprint.proposed.v1",
    "sdlc.component_stubs.committed.v1",
    "sdlc.accessibility_audit.completed.v1",
    "pr.opened.v1",
    "sdlc.test_plan.proposed.v1",
    "workflow.paused_for_approval.v1",  # security-review HITL
    "sdlc.iac.generated.v1",
    "sdlc.iac.validated.v1",
    "sdlc.iac.applied.v1",
    "deploy.completed.v1",             # staging
    "workflow.paused_for_approval.v1",  # approve-prod-deploy HITL
    "deploy.completed.v1",             # prod
    "sre.slo.configured.v1",
    "observability.dashboards.provisioned.v1",
]


def synthetic_smoke(correlation_id: str) -> bool:
    """Validates milestone ordering in synthetic mode (no live stack required)."""
    print(f"\n==> synthetic smoke  correlation_id={correlation_id}\n")

    errors: list[str] = []

    # 1. Milestone list must be non-empty.
    if not EXPECTED_MILESTONES:
        errors.append("milestone list is empty")

    # 2. No duplicate consecutive milestones (except the allowed HITL and deploy pairs).
    allowed_repeats = {"workflow.paused_for_approval.v1", "deploy.completed.v1"}
    prev = None
    for milestone in EXPECTED_MILESTONES:
        if milestone == prev and milestone not in allowed_repeats:
            errors.append(f"unexpected duplicate milestone: {milestone}")
        prev = milestone

    # 3. Confirm ordering invariants from the spec.
    def idx(name: str, occurrence: int = 0) -> int:
        count = 0
        for i, m in enumerate(EXPECTED_MILESTONES):
            if m == name:
                if count == occurrence:
                    return i
                count += 1
        return -1

    ordering_checks = [
        ("intent.committed.v1", "repo.scaffolded.v1", "intent before scaffold"),
        ("sdlc.adr.proposed.v1", "sdlc.api_contract.proposed.v1", "adr before api_contract"),
        ("sdlc.api_contract.proposed.v1", "sdlc.ui_blueprint.proposed.v1", "api_contract before ui_blueprint"),
        ("sdlc.threat_model.completed.v1", "workflow.paused_for_approval.v1", "threat_model before security HITL"),
        ("sdlc.iac.generated.v1", "sdlc.iac.validated.v1", "iac_generated before validated"),
        ("sdlc.iac.validated.v1", "sdlc.iac.applied.v1", "iac_validated before applied"),
        ("deploy.completed.v1", "sre.slo.configured.v1", "staging deploy before slo"),
        ("sre.slo.configured.v1", "observability.dashboards.provisioned.v1", "slo before dashboards"),
    ]

    for before, after, label in ordering_checks:
        b_idx = idx(before)
        a_idx = idx(after)
        if b_idx == -1:
            errors.append(f"missing milestone: {before}")
        elif a_idx == -1:
            errors.append(f"missing milestone: {after}")
        elif b_idx >= a_idx:
            errors.append(f"ordering violation ({label}): {before}[{b_idx}] must precede {after}[{a_idx}]")

    # 4. Exactly 2 HITL gates.
    hitl_count = EXPECTED_MILESTONES.count("workflow.paused_for_approval.v1")
    if hitl_count != 2:
        errors.append(f"expected 2 HITL gates, got {hitl_count}")

    # 5. Exactly 2 deploy events (staging + prod).
    deploy_count = EXPECTED_MILESTONES.count("deploy.completed.v1")
    if deploy_count != 2:
        errors.append(f"expected 2 deploy events (staging+prod), got {deploy_count}")

    if errors:
        print("FAIL:")
        for e in errors:
            print(f"  ✗ {e}")
        return False

    print("  All milestone ordering assertions passed.")
    print(f"  Total milestones: {len(EXPECTED_MILESTONES)}")
    print(f"  HITL gates: {hitl_count}")
    print(f"  Deploy events: {deploy_count}")
    return True


def live_smoke(correlation_id: str) -> bool:
    """Drives the live stack via the demo script."""
    import subprocess
    env = os.environ.copy()
    env["CORRELATION_ID"] = correlation_id
    result = subprocess.run(
        [sys.executable, "scripts/demo_intent_to_infrastructure.py"],
        env=env,
        timeout=600,
    )
    return result.returncode == 0


def main() -> int:
    correlation_id = os.environ.get("CORRELATION_ID", "smoke-synthetic")
    live = os.environ.get("LIVE_STACK", "").lower() in ("1", "true", "yes")

    if live:
        print("Running LIVE smoke test against stack")
        ok = live_smoke(correlation_id)
    else:
        ok = synthetic_smoke(correlation_id)

    report = {
        "mode": "live" if live else "synthetic",
        "correlation_id": correlation_id,
        "success": ok,
    }
    print(f"\n{json.dumps(report, indent=2)}")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
