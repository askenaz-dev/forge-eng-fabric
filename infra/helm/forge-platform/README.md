# forge-platform (umbrella chart)

Installs every Forge platform service in one shot. Composes 33 service charts plus the `retention-jobs` cron bundle as Helm subchart dependencies.

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
helm install forge-platform infra/helm/forge-platform \
  -f infra/helm/forge-platform/values-local.yaml

# Staging GKE
helm install forge-platform infra/helm/forge-platform \
  -f infra/helm/forge-platform/values-staging.yaml \
  --namespace forge \
  --create-namespace

# Production
helm install forge-platform infra/helm/forge-platform \
  -f infra/helm/forge-platform/values-prod.yaml \
  --namespace forge \
  --create-namespace \
  --set global.tier=medium
```

## Disabling a service

Every service has a `<svc>.enabled` toggle. Disable a service in a specific environment by setting it to `false`:

```sh
helm install forge-platform infra/helm/forge-platform \
  -f values-staging.yaml \
  --set rag-ingest.enabled=false
```

## Verifying chart signatures

In CI, the umbrella chart and every service chart are signed with Cosign. To verify before install:

```sh
cosign verify-blob \
  --certificate-identity-regexp '.*forge-eng-fabric.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --signature forge-platform-0.1.0.tgz.sig \
  forge-platform-0.1.0.tgz
```

See [`infra/helm/README.md`](../README.md) for details on key locations.

## Dependency list

| Service | Flavor |
|---|---|
| `alfred` | service-http |
| `app-onboarding` | service-http |
| `approvals` | service-http |
| `asset-observability` | service-http |
| `audit` | service-http |
| `control-plane` | service-http |
| `deploy-orchestrator` | service-http |
| `diagnosis` | service-worker |
| `eval-harness-adv` | service-http |
| `evolution` | service-worker |
| `finops` | service-http |
| `finops-advisor` | service-http |
| `healing-engine` | service-worker |
| `iac-drift` | service-http |
| `incident-detection` | service-worker |
| `incidents-kb` | service-http |
| `marketplace` | service-http |
| `mcp` | service-http |
| `openspec` | service-http |
| `permissions` | service-http |
| `policy-engine` | service-http |
| `postmortem` | service-worker |
| `prompt-registry` | service-http |
| `rag-ingest` | service-worker |
| `rag-query` | service-worker |
| `registry` | service-http |
| `runtime-registry` | service-http |
| `scaffolder` | service-http |
| `sdlc-orchestrator` | service-http |
| `traceability` | service-http |
| `webhooks` | service-http |
| `workflow-registry` | service-http |
| `workflow-runtime` | service-http |
| `retention-jobs` | service-cron |
