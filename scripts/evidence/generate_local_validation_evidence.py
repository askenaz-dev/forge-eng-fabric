from __future__ import annotations

import json
import subprocess
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
EVIDENCE_ROOT = ROOT / "docs" / "governance" / "evidence"


@dataclass(frozen=True)
class CommandSpec:
    phase: str
    name: str
    command: list[str]
    cwd: Path


COMMANDS = [
    CommandSpec("phase-1", "registry-lifecycle-and-invocation", ["go", "test", "./..."], ROOT / "services" / "registry" / "cmd" / "server"),
    CommandSpec("phase-2", "app-onboarding-flow", ["go", "test", "./..."], ROOT / "services" / "app-onboarding"),
    CommandSpec("phase-4", "sdlc-orchestrator", ["go", "test", "./..."], ROOT / "services" / "sdlc-orchestrator"),
    CommandSpec("phase-4", "traceability-graph", ["go", "test", "./..."], ROOT / "services" / "traceability"),
    CommandSpec("phase-4", "confluence-mcp-local", ["uv", "run", "pytest", "-q", "services/mcp/tests/test_confluence.py"], ROOT),
    CommandSpec("phase-5", "workflow-registry", ["go", "test", "./..."], ROOT / "services" / "workflow-registry"),
    CommandSpec("phase-5", "marketplace", ["go", "test", "./..."], ROOT / "services" / "marketplace"),
    CommandSpec("phase-5", "workflow-runtime", ["go", "test", "./..."], ROOT / "services" / "workflow-runtime"),
    CommandSpec("phase-5", "asset-observability", ["go", "test", "./..."], ROOT / "services" / "asset-observability"),
    CommandSpec("phase-5", "advanced-eval-harness", ["uv", "run", "--extra", "dev", "pytest", "-q", "tests/test_harness.py"], ROOT / "services" / "eval-harness-adv"),
]


TASK_COVERAGE = {
    "phase-1": [
        {
            "task": "13.5",
            "evidence": "Registry lifecycle tests cover failed promotion with low eval scores and successful T1 approval when thresholds pass.",
        },
        {
            "task": "13.6",
            "evidence": "Registry invocation tests cover prod-relevant blocking for in_review assets and the audit event com.forge.asset.invocation.checked.v1.",
        },
    ],
    "phase-2": [
        {
            "task": "11.1",
            "evidence": "App onboarding tests execute pilot-style onboarding flows through policy, scaffold, GitHub MCP stub, branch protection, required checks and asset registration.",
        },
        {
            "task": "11.2",
            "evidence": "App onboarding and registry tests verify audit/events, asset registration, required checks, image signature/SBOM lifecycle gate semantics, and OpenSpec-link check configuration.",
        },
    ],
    "phase-4": [
        {
            "task": "5.5",
            "evidence": "Confluence MCP local E2E covers create/update/attach/label/search plus OpenSpec header, forge-managed label and webhook event behavior.",
        },
        {
            "task": "11.1",
            "evidence": "SDLC orchestrator tests cover phase progression, gate blocking/override and metrics; traceability tests cover graph materialization from events.",
        },
    ],
    "phase-5": [
        {
            "task": "13.1",
            "evidence": "Workflow registry, marketplace and eval harness tests cover publish, eval pass, tenant approval/install, exact version pinning and install event emission.",
        },
        {
            "task": "13.2",
            "evidence": "Workflow registry/runtime/marketplace/eval/observability tests cover forge-certified workflow contracts, runtime behavior and asset metrics surfaces.",
        },
    ],
}


def _clip(value: str, limit: int = 12000) -> str:
    if len(value) <= limit:
        return value
    return value[:limit] + "\n... <truncated>"


def run_command(spec: CommandSpec) -> dict[str, object]:
    started = datetime.now(UTC)
    proc = subprocess.run(
        spec.command,
        cwd=spec.cwd,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )
    finished = datetime.now(UTC)
    return {
        "name": spec.name,
        "command": spec.command,
        "cwd": str(spec.cwd.relative_to(ROOT)),
        "started_at": started.isoformat(),
        "finished_at": finished.isoformat(),
        "exit_code": proc.returncode,
        "output": _clip(proc.stdout),
    }


def write_phase_evidence(phase: str, results: list[dict[str, object]], run_id: str) -> None:
    phase_dir = EVIDENCE_ROOT / phase
    phase_dir.mkdir(parents=True, exist_ok=True)
    passed = all(result["exit_code"] == 0 for result in results)
    payload = {
        "run_id": run_id,
        "phase": phase,
        "generated_at": datetime.now(UTC).isoformat(),
        "mode": "local-deterministic-validation",
        "passed": passed,
        "task_coverage": TASK_COVERAGE.get(phase, []),
        "commands": results,
    }
    (phase_dir / "local-validation.json").write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    if not passed:
        raise SystemExit(f"{phase} local validation failed; see {phase_dir / 'local-validation.json'}")


def write_bootstrap_evidence(run_id: str) -> None:
    phase_refs = {
        phase: str((EVIDENCE_ROOT / phase / "local-validation.json").relative_to(ROOT)).replace("\\", "/")
        for phase in ("phase-1", "phase-2", "phase-4", "phase-5")
    }
    payload = {
        "run_id": run_id,
        "phase": "bootstrap",
        "generated_at": datetime.now(UTC).isoformat(),
        "mode": "meta-change-reconciliation",
        "passed": True,
        "evidence_refs": phase_refs,
        "completed_phase_changes": [
            "phase-0-foundations (archived)",
            "phase-1-agentic-core",
            "phase-2-app-onboarding",
            "phase-3-deployable-apps",
            "phase-4-sdlc-orchestration",
            "phase-5-workflow-marketplace",
            "phase-6-autonomous-ops",
        ],
        "note": "bootstrap-forge-platform is a meta/umbrella change; concrete implementation evidence lives in the phase changes and referenced evidence files.",
    }
    phase_dir = EVIDENCE_ROOT / "bootstrap"
    phase_dir.mkdir(parents=True, exist_ok=True)
    (phase_dir / "local-validation.json").write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def main() -> None:
    run_id = datetime.now(UTC).strftime("local-%Y%m%dT%H%M%SZ")
    by_phase: dict[str, list[dict[str, object]]] = {}
    for spec in COMMANDS:
        by_phase.setdefault(spec.phase, []).append(run_command(spec))
    for phase, results in sorted(by_phase.items()):
        write_phase_evidence(phase, results, run_id)
    write_bootstrap_evidence(run_id)
    print(f"local validation evidence generated: {run_id}")


if __name__ == "__main__":
    main()
