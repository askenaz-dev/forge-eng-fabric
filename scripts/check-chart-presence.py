#!/usr/bin/env python3
"""Verify every Kubernetes-bound service has a Helm chart.

A service is "Kubernetes-bound" if its `services/<svc>/forge-service.yaml` declares
`spec.kubernetesBound: true`. For each such service, this script asserts a chart
exists at `infra/helm/<svc>/Chart.yaml`.

Exit codes:
  0 = all good
  1 = at least one Kubernetes-bound service missing a chart
  2 = invocation error (missing manifest, etc.)
"""

from __future__ import annotations

import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parent.parent
SERVICES = REPO / "services"
HELM = REPO / "infra" / "helm"


def main() -> int:
    if not SERVICES.exists():
        print(f"ERROR: services/ not found", file=sys.stderr)
        return 2
    missing: list[str] = []
    untagged: list[str] = []
    for svc_dir in sorted(p for p in SERVICES.iterdir() if p.is_dir()):
        manifest = svc_dir / "forge-service.yaml"
        if not manifest.exists():
            untagged.append(svc_dir.name)
            continue
        try:
            doc = yaml.safe_load(manifest.read_text(encoding="utf-8")) or {}
        except yaml.YAMLError as exc:
            print(f"ERROR: invalid YAML in {manifest}: {exc}", file=sys.stderr)
            return 2
        spec = doc.get("spec") or {}
        if not spec.get("kubernetesBound", False):
            continue
        chart_path = REPO / spec.get("helmChart", f"infra/helm/{svc_dir.name}") / "Chart.yaml"
        if not chart_path.exists():
            missing.append(f"{svc_dir.name} (expected {chart_path.relative_to(REPO)})")

    if untagged:
        print("WARN: services without forge-service.yaml (run scripts/gen-forge-service-manifests.py):")
        for svc in untagged:
            print(f"  - {svc}")

    if missing:
        print("FAIL: Kubernetes-bound services missing a Helm chart:")
        for entry in missing:
            print(f"  - {entry}")
        return 1
    print(f"OK: every Kubernetes-bound service has a chart ({len(list(SERVICES.iterdir()))} services scanned)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
