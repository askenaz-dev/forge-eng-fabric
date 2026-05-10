#!/usr/bin/env python3
"""Generate one Helm chart per Forge service from `forge-service.yaml`.

Each generated chart consumes the flavor library at `infra/helm/_flavors/<flavor>` and
exposes:
- Chart.yaml with the flavor as a Helm dependency
- values.yaml with sensible defaults plus environment overlay files (-staging, -prod)
- templates/all.yaml that includes the flavor's named templates
- README.md documenting usage

Idempotent: rewrites only when content differs.
"""

from __future__ import annotations

import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parent.parent
SERVICES = REPO / "services"
HELM = REPO / "infra" / "helm"

# Per-service overrides for values.yaml. Only fields that differ from the flavor default.
SERVICE_OVERRIDES: dict[str, dict] = {
    "alfred": {"image": {"repository": "forge/alfred"}, "service": {"port": 8085},
               "env": {"ADDR": ":8085", "ALFRED_DIALOGUE_API": "disabled"}},
    "app-onboarding": {"image": {"repository": "forge/app-onboarding"}, "service": {"port": 8101}},
    "approvals": {"image": {"repository": "forge/approvals"}, "service": {"port": 8104}},
    "asset-observability": {"image": {"repository": "forge/asset-observability"}, "service": {"port": 8113}},
    "audit": {"image": {"repository": "forge/audit-service"}, "service": {"port": 8083}},
    "control-plane": {"image": {"repository": "forge/control-plane"}, "service": {"port": 8081}},
    "deploy-orchestrator": {"image": {"repository": "forge/deploy-orchestrator"}, "service": {"port": 8112}},
    "eval-harness-adv": {"image": {"repository": "forge/eval-harness-adv"}, "service": {"port": 8121}},
    "finops": {"image": {"repository": "forge/finops"}, "service": {"port": 8122}, "replicaCount": 1},
    "finops-advisor": {"image": {"repository": "forge/finops-advisor"}, "service": {"port": 8126}, "replicaCount": 1},
    "iac-drift": {"image": {"repository": "forge/iac-drift"}, "service": {"port": 8125}, "replicaCount": 1},
    "incidents-kb": {"image": {"repository": "forge/incidents-kb"}, "service": {"port": 8131}, "replicaCount": 1},
    "marketplace": {"image": {"repository": "forge/marketplace"}, "service": {"port": 8118}},
    "mcp": {"image": {"repository": "forge/mcp"}, "service": {"port": 8089}},
    "openspec": {"image": {"repository": "forge/openspec"}, "service": {"port": 8084}},
    "permissions": {"image": {"repository": "forge/permissions"}, "service": {"port": 8086}},
    "policy-engine": {"image": {"repository": "forge/policy-engine"}, "service": {"port": 8088}},
    "prompt-registry": {"image": {"repository": "forge/prompt-registry"}, "service": {"port": 8124}, "replicaCount": 1},
    "registry": {"image": {"repository": "forge/registry"}, "service": {"port": 8082}},
    "runtime-registry": {"image": {"repository": "forge/runtime-registry"}, "service": {"port": 8110}},
    "scaffolder": {"image": {"repository": "forge/scaffolder"}, "service": {"port": 8102}},
    "sdlc-orchestrator": {"image": {"repository": "forge/sdlc-orchestrator"}, "service": {"port": 8120}},
    "traceability": {"image": {"repository": "forge/traceability"}, "service": {"port": 8123}, "replicaCount": 1},
    "webhooks": {"image": {"repository": "forge/webhooks"}, "service": {"port": 8103}},
    "workflow-registry": {"image": {"repository": "forge/workflow-registry"}, "service": {"port": 8117}},
    "workflow-runtime": {"image": {"repository": "forge/workflow-runtime"}, "service": {"port": 8119},
                         "replicaCount": 3},
    # Workers
    "diagnosis": {"image": {"repository": "forge/diagnosis"}},
    "evolution": {"image": {"repository": "forge/evolution"}},
    "healing-engine": {"image": {"repository": "forge/healing-engine"}},
    "incident-detection": {"image": {"repository": "forge/incident-detection"}},
    "postmortem": {"image": {"repository": "forge/postmortem"}},
    "rag-ingest": {"image": {"repository": "forge/rag-ingest"},
                   "resources": {"requests": {"cpu": "200m", "memory": "512Mi"},
                                  "limits": {"cpu": "1000m", "memory": "2Gi"}}},
    "rag-query": {"image": {"repository": "forge/rag-query"},
                  "resources": {"requests": {"cpu": "200m", "memory": "512Mi"},
                                 "limits": {"cpu": "1000m", "memory": "1Gi"}}},
}

# Skip charts that already have authored content we want to preserve. Initial release
# regenerates everything, so the set is empty by default.
PRESERVE_EXISTING: set[str] = set()


def chart_yaml(svc: str, flavor: str) -> str:
    return f"""apiVersion: v2
name: {svc}
description: Forge {svc} service
type: application
version: 0.1.0
appVersion: "0.1.0"
dependencies:
  - name: {flavor}
    version: 0.1.0
    repository: file://../_flavors/{flavor}
"""


def _flavor_defaults(flavor: str) -> dict:
    flavor_values = REPO / "infra" / "helm" / "_flavors" / flavor / "values.yaml"
    if not flavor_values.exists():
        return {}
    return yaml.safe_load(flavor_values.read_text(encoding="utf-8")) or {}


def _deep_merge(base: dict, overlay: dict) -> dict:
    out = dict(base)
    for key, value in overlay.items():
        if key in out and isinstance(out[key], dict) and isinstance(value, dict):
            out[key] = _deep_merge(out[key], value)
        else:
            out[key] = value
    return out


def values_yaml(svc: str, flavor: str) -> str:
    """Inline the flavor's default values plus per-service overrides.

    Helm library charts don't merge their `values.yaml` into the consuming
    chart automatically — the consumer must supply every key the templates
    reference. We pre-merge here so each service chart's values.yaml is
    self-contained and `helm template` works without extra flags.
    """
    base = _flavor_defaults(flavor)
    overrides = SERVICE_OVERRIDES.get(svc, {"image": {"repository": f"forge/{svc}"}})
    merged = _deep_merge(base, overrides)
    return yaml.safe_dump(merged, sort_keys=False, default_flow_style=False)


def values_overlay(svc: str, env: str) -> str:
    if env == "staging":
        return f"""# Staging overlay for {svc}. Tier: small.
replicaCount: 2
image:
  tag: staging
"""
    elif env == "prod":
        return f"""# Production overlay for {svc}. Tier: medium.
replicaCount: 3
image:
  tag: prod
resources:
  requests:
    cpu: 200m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 1Gi
"""
    else:
        return f"# Local overlay for {svc}.\nreplicaCount: 1\nimage:\n  tag: local\n"


def all_template_http() -> str:
    return """{{- include "service-http.deployment" . }}
---
{{- include "service-http.service" . }}
---
{{- include "service-http.serviceaccount" . }}
---
{{- include "service-http.hpa" . }}
---
{{- include "service-http.pdb" . }}
---
{{- include "service-http.networkpolicy" . }}
---
{{- include "service-http.servicemonitor" . }}
"""


def all_template_worker() -> str:
    return """{{- include "service-worker.deployment" . }}
---
{{- include "service-worker.serviceaccount" . }}
---
{{- include "service-worker.hpa" . }}
---
{{- include "service-worker.pdb" . }}
---
{{- include "service-worker.networkpolicy" . }}
---
{{- include "service-worker.servicemonitor" . }}
"""


def all_template_cron() -> str:
    return """{{- include "service-cron.cronjob" . }}
---
{{- include "service-cron.serviceaccount" . }}
---
{{- include "service-cron.networkpolicy" . }}
"""


def chart_readme(svc: str, flavor: str) -> str:
    install_cmd = f"""helm install {svc} infra/helm/{svc} \\
  --values infra/helm/{svc}/values.yaml \\
  --values infra/helm/{svc}/values-local.yaml"""
    return f"""# {svc}

Forge `{svc}` service Helm chart.

## Purpose

Deploys the Forge `{svc}` service. This chart is generated from the `{flavor}` flavor under `infra/helm/_flavors/{flavor}/`.

## Required values

| Key | Description |
|---|---|
| `image.repository` | Container image repository |
| `image.tag` | Container image tag |
| `service.port` | Service-specific port (HTTP flavor only) |

## Optional values

See [`values.yaml`](values.yaml) for defaults. Common overrides:

| Key | Default | Description |
|---|---|---|
| `replicaCount` | 2 | Replica count |
| `resources.*` | per flavor | Requests / limits |
| `autoscaling.enabled` | true | HPA enabled |
| `podDisruptionBudget.enabled` | true | PDB enabled |
| `networkPolicy.enabled` | true | Deny-by-default NetworkPolicy enabled |
| `serviceMonitor.enabled` | true | ServiceMonitor for Prometheus scraping |

## Environment overlays

- [`values-local.yaml`](values-local.yaml) — local development
- [`values-staging.yaml`](values-staging.yaml) — staging cluster (tier=small)
- [`values-prod.yaml`](values-prod.yaml) — production (tier=medium by default)

Refer to [`docs/platform-enablement.md` Hardware & Sizing](../../../docs/platform-enablement.md#hardware--sizing) for the canonical sizing table.

## Dependencies

| Dependency | Version | Source |
|---|---|---|
| `{flavor}` | 0.1.0 | `infra/helm/_flavors/{flavor}/` (file:// dep) |

## Install

```sh
{install_cmd}
```

For a multi-environment install, use the umbrella chart at `infra/helm/forge-platform/`.

## Sign and verify

This chart is signed with Cosign in the release pipeline. To verify:

```sh
cosign verify-blob \\
  --certificate-identity-regexp '.*forge-eng-fabric.*' \\
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \\
  --signature {svc}-0.1.0.tgz.sig \\
  {svc}-0.1.0.tgz
```
"""


def write_if_changed(path: Path, content: str) -> bool:
    if path.exists() and path.read_text(encoding="utf-8") == content:
        return False
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return True


def gen_chart(svc: str, flavor: str) -> int:
    if svc in PRESERVE_EXISTING:
        return 0
    chart_dir = HELM / svc
    # Wipe stale per-resource templates left over from older hand-authored
    # charts; the generator owns templates/all.yaml as the single source.
    templates_dir = chart_dir / "templates"
    if templates_dir.exists():
        for stale in templates_dir.iterdir():
            if stale.name == "all.yaml":
                continue
            if stale.is_file():
                stale.unlink()
    written = 0
    if write_if_changed(chart_dir / "Chart.yaml", chart_yaml(svc, flavor)):
        written += 1
    if write_if_changed(chart_dir / "values.yaml", values_yaml(svc, flavor)):
        written += 1
    if write_if_changed(chart_dir / "values-local.yaml", values_overlay(svc, "local")):
        written += 1
    if write_if_changed(chart_dir / "values-staging.yaml", values_overlay(svc, "staging")):
        written += 1
    if write_if_changed(chart_dir / "values-prod.yaml", values_overlay(svc, "prod")):
        written += 1
    if flavor == "service-http":
        tpl = all_template_http()
    elif flavor == "service-worker":
        tpl = all_template_worker()
    else:
        tpl = all_template_cron()
    if write_if_changed(chart_dir / "templates" / "all.yaml", tpl):
        written += 1
    if write_if_changed(chart_dir / "README.md", chart_readme(svc, flavor)):
        written += 1
    return written


def main() -> int:
    total_written = 0
    chart_count = 0
    for svc_dir in sorted(p for p in SERVICES.iterdir() if p.is_dir()):
        manifest = svc_dir / "forge-service.yaml"
        if not manifest.exists():
            continue
        doc = yaml.safe_load(manifest.read_text(encoding="utf-8")) or {}
        spec = doc.get("spec") or {}
        if not spec.get("kubernetesBound", False):
            continue
        flavor = spec.get("flavor")
        if flavor not in {"service-http", "service-worker", "service-cron"}:
            print(f"WARN: {svc_dir.name} has unknown flavor {flavor!r}; skipping", file=sys.stderr)
            continue
        written = gen_chart(svc_dir.name, flavor)
        chart_count += 1
        total_written += written
        if written:
            print(f"updated chart: {svc_dir.name} ({written} files)")
    print(f"OK: {chart_count} charts processed, {total_written} files written")
    return 0


if __name__ == "__main__":
    sys.exit(main())
