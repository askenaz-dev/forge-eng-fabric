# IaC Drift

`services/iac-drift` runs hourly against Provisioned IaC workspaces and executes `terraform plan -detailed-exitcode` through its planner interface.

Exit code `2` creates `iac_drift_finding` records and emits `iac.drift.detected.v1` unless the finding is explicitly suppressed by `.forge-drift-ignore.yaml`.

Suppressions require:

```yaml
suppressions:
  - resource_pattern: "google_container_node_pool.*"
    field_pattern: "node_count"
    reason: "temporary autoscaler validation"
    expires_at: "2026-06-01T00:00:00Z"
```

Alfred uses the `propose-drift-remediation` skill to open remediation PRs and link them back to `iac_drift_finding.remediation_pr_url`.
