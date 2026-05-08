# Advanced eval harness

The advanced harness extends the Phase-1 eval harness with:

- Golden datasets registered as `eval_dataset` Registry assets.
- Regression gates that block publication when a key metric drops more
  than a configurable Δ vs the previous passing run.
- A/B comparison between two versions for opt-in Workspaces.
- Business-metric instrumentation declared per-workflow via
  `metadata.success_metric`.

Service: [`services/eval-harness-adv/`](../../services/eval-harness-adv).

## Lifecycle

1. **Register a dataset** with `POST /v1/datasets`. Datasets are immutable
   per version; edits require a new SemVer.
2. **Start a regression run** with `POST /v1/runs/regression`. The runner
   ingests the dataset, executes the workflow version under test, and
   records per-item outcomes via `POST /v1/runs/{run_id}/outcome`.
3. **Outcome classification**:
   - If `baseline_value − metric_value > delta_threshold`, outcome is
     `regression_blocked` and `workflow.publish.regression_blocked.v1`
     fires.
   - Otherwise outcome is `passed`.
4. **Publish gate** consults `GET /v1/workflows/{id}/publish_allowed?version=X`
   to decide whether `workflow:publish` is allowed.

## A/B

`POST /v1/runs/ab` opens an A/B run between two versions. Workspaces opt in
by tagging executions with the variant. After `target_executions` outcomes
are recorded, the runner reports the comparison:

- per-variant success rate, latency, cost, business metric
- significance using a two-proportion z-test (`p < 0.05`)

`workflow.eval.ab_completed.v1` carries the comparison.

## Business metrics

`metadata.success_metric` on the workflow names a business KPI (e.g.
`pr_merged_within_24h`, `incident_resolved_within_15m`). The harness records
this metric alongside the technical metrics on each run; it surfaces in the
per-asset Observability tab.

## Storage

Runs persist into `workflow_eval_run` (see
[migration](../../db/migrations/eval-harness-adv/0001_eval_harness.sql)).
The asset registry references runs through the
`asset_workflow_eval_run` table created in
`db/migrations/registry/0005_phase5_workflow_subresources.sql`.
