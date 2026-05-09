# Forge Helm Charts

This directory contains every Helm chart used to deploy Forge Engineering Fabric.

## Layout

| Path | Type | Notes |
|---|---|---|
| `_flavors/service-http/` | library | HTTP service flavor (Deployment, Service, HPA, PDB, NetworkPolicy, ServiceMonitor) |
| `_flavors/service-worker/` | library | Worker flavor (Deployment, HPA, PDB, NetworkPolicy, metrics-only ServiceMonitor) |
| `_flavors/service-cron/` | library | Cron flavor (CronJob, ServiceAccount, NetworkPolicy) |
| `<svc>/` | application | One per service in `services/<svc>/` |
| `retention-jobs/` | application | Retention/archival cron jobs (audit partitioning, audit archival, RAG reclassification) |
| `forge-platform/` | application (umbrella) | Composes every service chart with tier presets |

See [`_flavors/README.md`](_flavors/README.md) for the canonical flavor template documentation, and the per-chart README inside each directory for service-specific notes.

## Lint and verify

```sh
make helm-lint
```

This runs `helm lint` on every chart and verifies the rendered output contains the required resources for the chart's declared flavor (Deployment+Service+HPA+PDB+NetworkPolicy+ServiceMonitor for HTTP services, etc.). The script lives at [`scripts/helm-lint.sh`](../../scripts/helm-lint.sh).

## Sizing alignment

The umbrella chart's `values-staging.yaml` and `values-prod.yaml` mirror the canonical sizing table in [`docs/platform-enablement.md`](../../docs/platform-enablement.md#hardware--sizing). The CI check at `make sizing-check` ([scripts/check-sizing.py](../../scripts/check-sizing.py)) fails if the umbrella values diverge from the table.

## Releasing signed charts

The release pipeline at [`.github/workflows/helm-release.yml`](../../.github/workflows/helm-release.yml) packages every chart as a `.tgz` and signs it with Cosign (keyless, GitHub OIDC). To produce a release:

1. Tag the repo with `helm-v<version>` (e.g., `helm-v0.1.0`).
2. Push the tag — GitHub Actions packages and signs every chart.
3. The signed `.tgz` and `.sig` artifacts are attached to the GitHub Release.

### Verifying a chart signature

```sh
cosign verify-blob \
  --certificate-identity-regexp '.*forge-eng-fabric.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --signature <chart>-<version>.tgz.sig \
  --certificate <chart>-<version>.tgz.pem \
  <chart>-<version>.tgz
```

### Key location and rotation

Cosign uses **keyless** signing via Fulcio + the GitHub OIDC issuer. There is no static private key to rotate; the trust root is the OIDC identity (`https://token.actions.githubusercontent.com`) plus the certificate identity regex (`.*forge-eng-fabric.*`).

If a fork or downstream repo needs to sign with the same identity, configure that repo's `id-token: write` permission and reuse the workflow.

## Regenerating service charts

If you change a service's flavor in `services/<svc>/forge-service.yaml`, regenerate its chart:

```sh
python scripts/gen-helm-charts.py
python scripts/gen-umbrella-chart.py
make helm-lint
```

The generators are idempotent and only rewrite changed files.
