#!/usr/bin/env python3
"""Generate the umbrella chart `infra/helm/forge-platform/` referencing every service chart.

Reads `services/<svc>/forge-service.yaml` to discover Kubernetes-bound services and emits:
- `Chart.yaml` with one Helm dependency per service chart
- `values.yaml` with global defaults and per-service stubs
- `values-local.yaml` / `values-staging.yaml` / `values-prod.yaml` with tier presets
- `README.md`

Idempotent.
"""

from __future__ import annotations

import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parent.parent
SERVICES = REPO / "services"
HELM = REPO / "infra" / "helm"
UMBRELLA = HELM / "forge-platform"


def collect_services() -> list[tuple[str, str]]:
    out: list[tuple[str, str]] = []
    for svc_dir in sorted(p for p in SERVICES.iterdir() if p.is_dir()):
        manifest = svc_dir / "forge-service.yaml"
        if not manifest.exists():
            continue
        doc = yaml.safe_load(manifest.read_text(encoding="utf-8")) or {}
        spec = doc.get("spec") or {}
        if not spec.get("kubernetesBound", False):
            continue
        flavor = spec.get("flavor", "service-http")
        out.append((svc_dir.name, flavor))
    return out


def chart_yaml(services: list[tuple[str, str]]) -> str:
    lines = [
        "apiVersion: v2",
        "name: forge-platform",
        "description: Umbrella chart that installs every Forge platform service",
        "type: application",
        "version: 0.1.0",
        "appVersion: \"0.1.0\"",
        "dependencies:",
    ]
    for svc, _ in services:
        lines.append(f"  - name: {svc}")
        lines.append(f"    version: 0.1.0")
        lines.append(f"    repository: file://../{svc}")
        lines.append(f"    condition: {svc}.enabled")
    lines.append("  - name: retention-jobs")
    lines.append("    version: 0.1.0")
    lines.append("    repository: file://../retention-jobs")
    lines.append("    condition: retention-jobs.enabled")
    return "\n".join(lines) + "\n"


def values_yaml(services: list[tuple[str, str]]) -> str:
    lines = [
        "# Umbrella defaults. Override per environment via -f values-<env>.yaml.",
        "global:",
        "  tier: small  # small | medium | large",
        "",
    ]
    for svc, _ in services:
        lines.append(f"{svc}:")
        lines.append(f"  enabled: true")
        lines.append("")
    lines.append("retention-jobs:")
    lines.append("  enabled: true")
    return "\n".join(lines) + "\n"


def values_overlay(services: list[tuple[str, str]], env: str) -> str:
    if env == "local":
        head = ["# Local overlay: minimal replicas, no autoscaling, no PDB.",
                "global:",
                "  tier: small",
                ""]
        per_service = (
            "  enabled: true\n"
            "  replicaCount: 1\n"
            "  autoscaling:\n    enabled: false\n"
            "  podDisruptionBudget:\n    enabled: false\n"
            "  serviceMonitor:\n    enabled: false\n"
        )
    elif env == "staging":
        head = ["# Staging overlay: tier=small, autoscale enabled, single-zone.",
                "global:",
                "  tier: small",
                ""]
        per_service = (
            "  enabled: true\n"
            "  replicaCount: 2\n"
            "  image:\n    tag: staging\n"
        )
    else:  # prod
        head = ["# Production overlay: tier=medium by default; flip to large for big BUs.",
                "global:",
                "  tier: medium",
                ""]
        per_service = (
            "  enabled: true\n"
            "  replicaCount: 3\n"
            "  image:\n    tag: prod\n"
            "  resources:\n"
            "    requests:\n      cpu: 200m\n      memory: 512Mi\n"
            "    limits:\n      cpu: 1000m\n      memory: 1Gi\n"
        )

    out = "\n".join(head) + "\n"
    for svc, _ in services:
        out += f"{svc}:\n{per_service}\n"
    out += "retention-jobs:\n  enabled: true\n"
    return out


def readme(services: list[tuple[str, str]]) -> str:
    return f"""# forge-platform (umbrella chart)

Installs every Forge platform service in one shot. Composes {len(services)} service charts plus the `retention-jobs` cron bundle as Helm subchart dependencies.

## Tier presets

The `global.tier` value selects pre-baked sizing per the [Hardware & Sizing table](../../../docs/platform-enablement.md#hardware--sizing).

| `global.tier` | Profile | Use when |
|---|---|---|
| `small` | ≤10 apps | Staging or small-BU production |
| `medium` | ≤50 apps | Mid-size BU production |
| `large` | ≤200 apps | Large-BU production |

## Install

```sh
# Local (Minikube or kind)
helm dependency update infra/helm/forge-platform
helm install forge-platform infra/helm/forge-platform \\
  -f infra/helm/forge-platform/values-local.yaml

# Staging GKE
helm install forge-platform infra/helm/forge-platform \\
  -f infra/helm/forge-platform/values-staging.yaml \\
  --namespace forge \\
  --create-namespace

# Production
helm install forge-platform infra/helm/forge-platform \\
  -f infra/helm/forge-platform/values-prod.yaml \\
  --namespace forge \\
  --create-namespace \\
  --set global.tier=medium
```

## Disabling a service

Every service has a `<svc>.enabled` toggle. Disable a service in a specific environment by setting it to `false`:

```sh
helm install forge-platform infra/helm/forge-platform \\
  -f values-staging.yaml \\
  --set rag-ingest.enabled=false
```

## Verifying chart signatures

In CI, the umbrella chart and every service chart are signed with Cosign. To verify before install:

```sh
cosign verify-blob \\
  --certificate-identity-regexp '.*forge-eng-fabric.*' \\
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \\
  --signature forge-platform-0.1.0.tgz.sig \\
  forge-platform-0.1.0.tgz
```

See [`infra/helm/README.md`](../README.md) for details on key locations.

## Dependency list

| Service | Flavor |
|---|---|
""" + "\n".join([f"| `{svc}` | {flavor} |" for svc, flavor in services]) + "\n| `retention-jobs` | service-cron |\n"


def write_if_changed(path: Path, content: str) -> bool:
    if path.exists() and path.read_text(encoding="utf-8") == content:
        return False
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return True


def main() -> int:
    services = collect_services()
    if not services:
        print("ERROR: no Kubernetes-bound services discovered", file=sys.stderr)
        return 2
    UMBRELLA.mkdir(parents=True, exist_ok=True)
    written = 0
    if write_if_changed(UMBRELLA / "Chart.yaml", chart_yaml(services)):
        written += 1
    if write_if_changed(UMBRELLA / "values.yaml", values_yaml(services)):
        written += 1
    for env in ("local", "staging", "prod"):
        if write_if_changed(UMBRELLA / f"values-{env}.yaml", values_overlay(services, env)):
            written += 1
    if write_if_changed(UMBRELLA / "README.md", readme(services)):
        written += 1
    print(f"OK: forge-platform umbrella chart updated, {len(services)} services, {written} files written")
    return 0


if __name__ == "__main__":
    sys.exit(main())
