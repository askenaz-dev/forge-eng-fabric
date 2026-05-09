# Forge Helm Flavor Templates

Three library charts under `_flavors/` provide the canonical templates for every service chart in `infra/helm/`. They enforce mechanical uniformity across the ~33 service charts so reviewers can read all of them confidently.

| Flavor | When to use | Resources rendered |
|---|---|---|
| `service-http` | FastAPI / Next.js / Go HTTP services | Deployment, Service, ServiceAccount (+ RBAC), HPA, PDB, NetworkPolicy, ServiceMonitor |
| `service-worker` | Event consumers / Kafka or queue workers without inbound HTTP | Deployment, ServiceAccount (+ RBAC), HPA, PDB, NetworkPolicy, ServiceMonitor (metrics-only Service) |
| `service-cron` | Scheduled jobs (retention, archival, partitioning) | CronJob, ServiceAccount (+ RBAC), NetworkPolicy |

## Consuming a flavor

A service chart uses a flavor by declaring it as a Helm dependency and including the named templates.

`infra/helm/<svc>/Chart.yaml`:

```yaml
apiVersion: v2
name: <svc>
description: Forge <svc> service
type: application
version: 0.1.0
appVersion: "0.1.0"
dependencies:
  - name: service-http
    version: 0.1.0
    repository: file://../_flavors/service-http
```

`infra/helm/<svc>/templates/all.yaml`:

```yaml
{{ include "service-http.deployment" . }}
---
{{ include "service-http.service" . }}
---
{{ include "service-http.serviceaccount" . }}
---
{{ include "service-http.hpa" . }}
---
{{ include "service-http.pdb" . }}
---
{{ include "service-http.networkpolicy" . }}
---
{{ include "service-http.servicemonitor" . }}
```

`infra/helm/<svc>/values.yaml` overrides only what differs from the flavor default. Image, port, env, and resources are typical overrides.

## Escape hatch

If a service genuinely cannot fit a flavor (e.g., needs a Statefulset for stable hostnames), it MAY add additional templates in its own `templates/` directory alongside the flavor inclusions. The flavor inclusions remain so the baseline (PDB, NetworkPolicy, ServiceMonitor) is enforced; the extra templates are reviewed as a separate concern.

This escape hatch is documented as a [risk in the platform-gaps-closure design](../../openspec/changes/platform-gaps-closure/design.md#risks--trade-offs).

## Validation

Run `make helm-lint` to lint every chart and verify the required resources are present per flavor.
