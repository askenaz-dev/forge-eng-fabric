# Observability Runbook

## Phase 0 Verification

1. Start the local stack with `make up`.
2. Run `make smoke` to generate Control Plane, Registry, Audit, Alfred and LiteLLM traffic.
3. Open Grafana at `http://localhost:3001`.
4. Confirm Prometheus has targets for `otel-collector`, Forge services and `litellm`.
5. Open Explore with the Tempo datasource and query recent traces. Use the service map view to confirm cross-service edges from HTTP traffic.
6. Open Explore with Loki and filter on standardized resource labels: `service.name`, `deployment.environment`, `forge.tenant_id`, `forge.workspace_id`, `forge.correlation_id`.

## Standard Labels

Forge services should set these OpenTelemetry resource or log attributes:

- `service.name`
- `deployment.environment`
- `forge.tenant_id`
- `forge.workspace_id`
- `forge.correlation_id`

The collector maps these to Loki resource labels through `loki.resource.labels` and derives Tempo service graph metrics with the `servicegraph` connector.

## Phase 2 App Onboarding

The app onboarding service exposes Prometheus metrics at `/metrics`; the local stack scrapes it as job `app-onboarding`.

Required Phase 2 metrics:

- `onboarding_duration_seconds`
- `onboarding_success_rate`
- `pipeline_gate_failure_rate`
- `pr_openspec_link_coverage`
- `image_signing_rate`
- `override_count`

SLOs and alerts are provisioned in `deploy/compose/prometheus/rules/phase-2-app-onboarding.yml`:

- Onboarding p95 must be <= 5 minutes.
- Image signing rate on `main` must be 100%.
- OpenSpec link coverage must be >= 95% for medium or higher criticality repos.

Grafana provisions `Phase 2 App Onboarding` from `deploy/compose/grafana/dashboards/phase-2-app-onboarding.json` for global tracking. Workspace-specific filtering should be added once production metric labels include `forge.workspace_id` on every emitted metric.
