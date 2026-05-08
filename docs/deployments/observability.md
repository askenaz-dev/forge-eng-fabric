# Deployment Observability

Phase 3 deployment services emit these metrics by workspace, environment, runtime type, and outcome:

- `deploy_duration_seconds`: histogram from request acceptance through notify.
- `deploy_success_rate`: rolling ratio of completed deployments over requested deployments.
- `image_verification_failure_rate`: ratio of failed `deployment.image_verified.v1` outcomes.
- `drift_findings_total`: counter of unsuppressed IaC drift findings by severity.
- `rollback_rate`: ratio of rollback deployments over completed forward deployments.
- `time_to_recover_seconds`: duration from `deployment.failed.v1` to `deployment.rolled_back.v1` or next successful deploy.

Initial SLOs:

- Dev deploy success rate: at least 95%.
- Prod deploy success rate: at least 99%.
- Image verification coverage: 100% of non-overridden deploys.
- Drift detection latency: p95 at or below 1 hour.

Grafana dashboard panels should group by `workspace_id`, `env`, and `runtime_id` and link deployment panels to audit by `correlation_id`.
