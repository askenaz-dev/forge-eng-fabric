#!/usr/bin/env python3
"""Generate `services/<svc>/forge-service.yaml` for every service.

Each manifest tags the service with its Helm flavor (`service-http`, `service-worker`,
or `service-cron`) and declares whether it must be Kubernetes-bound. Idempotent: writes
files only when content differs.

Run:
    python scripts/gen-forge-service-manifests.py
"""

from __future__ import annotations

import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
SERVICES = REPO / "services"

HTTP_SERVICES = {
    "alfred",
    "app-onboarding",
    "approvals",
    "asset-observability",
    "audit",
    "control-plane",
    "deploy-orchestrator",
    "eval-harness-adv",
    "finops",
    "finops-advisor",
    "iac-drift",
    "incidents-kb",
    "marketplace",
    "mcp",
    "openspec",
    "permissions",
    "policy-engine",
    "prompt-registry",
    "registry",
    "runtime-registry",
    "scaffolder",
    "sdlc-orchestrator",
    "traceability",
    "webhooks",
    "workflow-registry",
    "workflow-runtime",
}

WORKER_SERVICES = {
    "diagnosis",
    "evolution",
    "healing-engine",
    "incident-detection",
    "postmortem",
    "rag-ingest",
    "rag-query",
}

CRON_SERVICES: set[str] = set()


def manifest_for(svc: str, flavor: str) -> str:
    return f"""# Forge service manifest. Owned by Platform Architecture; consumed by build, helm-lint
# and the chart-presence check (`scripts/check-chart-presence.py`).
apiVersion: forge.platform/v1
kind: ServiceManifest
metadata:
  name: {svc}
spec:
  flavor: {flavor}
  kubernetesBound: true
  helmChart: infra/helm/{svc}
"""


def main() -> int:
    if not SERVICES.exists():
        print(f"ERROR: services/ not found at {SERVICES}", file=sys.stderr)
        return 2

    services_present = {p.name for p in SERVICES.iterdir() if p.is_dir()}
    declared = HTTP_SERVICES | WORKER_SERVICES | CRON_SERVICES
    missing_in_repo = declared - services_present
    if missing_in_repo:
        print(
            f"WARN: declared services not present in services/: {sorted(missing_in_repo)}",
            file=sys.stderr,
        )
    untagged = services_present - declared
    if untagged:
        print(f"WARN: services without a flavor declaration: {sorted(untagged)}", file=sys.stderr)

    written = 0
    for svc in sorted(services_present):
        if svc in HTTP_SERVICES:
            flavor = "service-http"
        elif svc in WORKER_SERVICES:
            flavor = "service-worker"
        elif svc in CRON_SERVICES:
            flavor = "service-cron"
        else:
            continue
        target = SERVICES / svc / "forge-service.yaml"
        new = manifest_for(svc, flavor)
        if target.exists() and target.read_text(encoding="utf-8") == new:
            continue
        target.write_text(new, encoding="utf-8")
        written += 1
        print(f"wrote {target.relative_to(REPO)}")
    print(f"OK: {written} manifest(s) written; {len(services_present)} services scanned")
    return 0


if __name__ == "__main__":
    sys.exit(main())
