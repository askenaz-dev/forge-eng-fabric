# Spec Delta: sdlc-finops (ADDED)

## ADDED Requirements

### Requirement: FinOps skills and budgets

The capability SHALL expose `estimate-cost-from-spec`, `monitor-budget`, `propose-cost-reduction` as registered skills, and persist budgets as `finops_budget` per Workspace/initiative.

#### Scenario: Estimate cost from spec produces structured forecast

- **GIVEN** an OpenSpec with target traffic and storage
- **WHEN** Alfred invokes `estimate-cost-from-spec`
- **THEN** the output MUST contain monthly cost estimate by category (compute, storage, network, LLM)
- **AND** confidence range and assumptions
- **AND** be stored linked to the initiative

### Requirement: Budget alerts at thresholds

Budgets MUST emit alert events at 50%, 80%, 100% consumption.

#### Scenario: Threshold alert emitted

- **GIVEN** a budget of $1000/month for `initiative-3`
- **WHEN** consumption crosses $800 (80%)
- **THEN** event `finops.budget.threshold_reached.v1` MUST be emitted with `threshold=80`
- **AND** the Approvals Inbox MUST receive an entry if the policy `hard-budget` is active

### Requirement: FinOps gates

Gates `cost_estimate_within_budget`, `llm_budget_within_limit` MUST be evaluated before initiative reaches `done`.

#### Scenario: Initiative cannot complete over LLM budget

- **GIVEN** an initiative whose LLM consumption exceeds the limit
- **WHEN** completion is requested
- **THEN** gate `llm_budget_within_limit` MUST fail
- **AND** Alfred MUST invoke `propose-cost-reduction` with concrete suggestions (caching, model downgrade, prompt simplification)
