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
