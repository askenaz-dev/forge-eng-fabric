# SDLC Observability

Phase 4 exposes the following initial metrics:

- `sdlc_phase_duration_seconds`
- `gate_pass_rate`
- `phase_block_rate`
- `traceability_coverage`
- `traceability_query_latency_p95`
- `jira_sync_lag_seconds`
- `confluence_sync_lag_seconds`

Initial SLOs:

- Gate evaluation p95 must stay at or below 5 seconds.
- Traceability query p95 must stay at or below 1 second.
- Jira and Confluence sync lag must stay at or below 5 minutes.

Grafana dashboard: `deploy/compose/grafana/dashboards/phase-4-sdlc-workspace.json`.
Prometheus rules: `deploy/compose/prometheus/rules/phase-4-sdlc.yml`.
