# finops-advisor

Forge `finops-advisor` service Helm chart.

## Purpose

Deploys the Forge `finops-advisor` service. This chart is generated from the `service-http` flavor under `infra/helm/_flavors/service-http/`.

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
| `service-http` | 0.1.0 | `infra/helm/_flavors/service-http/` (file:// dep) |

## Install

```sh
helm install finops-advisor infra/helm/finops-advisor \
  --values infra/helm/finops-advisor/values.yaml \
  --values infra/helm/finops-advisor/values-local.yaml
```

For a multi-environment install, use the umbrella chart at `infra/helm/forge-platform/`.

## Sign and verify

This chart is signed with Cosign in the release pipeline. To verify:

```sh
cosign verify-blob \
  --certificate-identity-regexp '.*forge-eng-fabric.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --signature finops-advisor-0.1.0.tgz.sig \
  finops-advisor-0.1.0.tgz
```
