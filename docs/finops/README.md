# FinOps Integration

Forge attributes cloud and LLM costs to SDLC initiatives through `initiative_openspec`.

## Required Billing Tags

- `workspace`
- `env`
- `asset`
- `initiative_openspec`

## Sources

- GCP Billing export is imported from BigQuery rows carrying the required tags.
- Langfuse and LiteLLM records are imported as LLM cost records and aggregated by initiative.

## Budgets

`finops_budget` stores a monthly limit per Workspace and initiative. Default thresholds are `50`, `80`, and `100` percent. Crossing a threshold emits `finops.budget.threshold_reached.v1` once per threshold.
