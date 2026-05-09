# retention-jobs

CronJobs that enforce the data retention policy in [`docs/governance/data-retention.md`](../../../docs/governance/data-retention.md).

This chart bundles three jobs:

| Job | Schedule (default) | Purpose |
|---|---|---|
| `audit-partition` | `0 1 * * *` | Roll monthly partitions on `audit_event` |
| `audit-archival` | `0 2 * * *` | Export expired partitions to GCS as Parquet, drop from Postgres |
| `rag-reclassify` | `0 3 * * *` | Re-evaluate retention deadlines on Milvus vectors when source classification changes |

## Required values per job

| Key | Description |
|---|---|
| `<job>.image.repository` | Container image repository |
| `<job>.image.tag` | Container image tag |
| `<job>.schedule` | Cron schedule |
| `<job>.env.JOB` | Job kind (`partition-audit`, `archive-audit`, `reclassify`) |
| `<job>.env.ENFORCE_RETENTION` | `false` (default) for dry-run mode; `true` enables deletion |

## Install

```sh
helm install forge-retention infra/helm/retention-jobs \
  --values infra/helm/retention-jobs/values.yaml
```

For environment-specific schedules and enforcement, use the umbrella chart at `infra/helm/forge-platform/`.

## Dry-run mode

By default, every job runs in dry-run mode. To enable enforcement:

1. Confirm the job has accumulated **two weeks of dry-run logs** (see [retention policy](../../../docs/governance/data-retention.md#dry-run-mode)).
2. Get co-approval from Platform and Security on the PR setting `<job>.env.ENFORCE_RETENTION=true`.
3. Roll the new value via `helm upgrade`.
