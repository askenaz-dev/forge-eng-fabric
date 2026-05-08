# FinOps Recommendations

`services/finops-advisor` (Python) runs a daily cron over BigQuery cost data
and produces cost-reduction recommendations. Each recommendation is paired
with a draft PR.

## Pattern detectors

| Detector | Trigger | Default thresholds |
|----------|---------|--------------------|
| `IdleResourceDetector` | cloud resources with utilization < 5% and spend ≥ $5/day | utilization, daily spend |
| `OversizedResourceDetector` | cloud resources running at 5%–30% utilization with spend ≥ $10/day | utilization, daily spend |
| `ExpensiveLLMSkillDetector` | aggregate skill spend > $200/period | total spend |
| `CacheablePromptDetector` | LLM skills with cache hit rate < 10% over ≥100 invocations | hit rate, invocations |

Thresholds are configurable per Workspace — production overrides them via
the policy-engine (template `finops-advisor-thresholds`).

## Recommendation kinds

- `idle_resource` — propose deletion or schedule-based scaling.
- `oversized_resource` — propose Terraform downsize.
- `expensive_llm_skill` — propose model downgrade or prompt simplification.
- `cacheable_prompt` — propose cache TTL bump or prompt normalization.

Every recommendation includes:

- `expected_savings_usd_monthly` — based on observed spend.
- `pr_url` + `pr_status` — URL of the draft PR opened by the
  `propose-cost-reduction` skill.
- `affected_resources` — the resource / skill / asset list.

## PR contract

PRs respect the standard Phase 2 + Phase 4 gates:

- forge/lint, forge/test-with-coverage
- forge/sast, forge/sca, forge/sbom
- forge/cosign-sign-attest
- forge/openspec-link
- finops/savings-realised (Phase 4 gate that verifies the PR claims a real
  saving)

The advisor never merges — humans approve the PR via the standard Approvals
Inbox flow.

## Endpoints

```
POST /v1/finops/run                 — manually run the advisor
GET  /v1/finops/recommendations     — list recommendations
                                      (filter by tenant_id; returns total
                                      savings + by_kind summary)
```

## Events

- `finops.recommendation.created.v1` — emitted per recommendation.

## Portal

The "FinOps Recommendations" module surfaces:
- total expected monthly savings,
- the by-kind breakdown,
- per-recommendation detail (with PR link).

Recommendations from synthetic flows carry `synthetic = true` and are
filtered out of headline savings totals.
