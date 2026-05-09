## MODIFIED Requirements

### Requirement: Lifecycle transitions are deferred to Fase 1
The Registry SHALL implement the **full asset lifecycle**: `proposed → in_review → approved → deprecated → retired`. Transitions SHALL be auditable. Only `approved` assets SHALL be invocable in production-relevant flows. Promotion to `approved` SHALL require eval scores meeting the threshold defined for the asset's trust level. Promotion of T4 assets SHALL require DevOps/SRE review; promotion of T5 (Critical/Core) assets SHALL require explicit SDLC Team approval. Deprecated assets SHALL remain discoverable with a deprecation banner and recommended replacement.

#### Scenario: Promotion blocked by failing evals
- **WHEN** an owner tries to promote an asset whose eval scores are below the threshold for its trust level
- **THEN** the promotion is rejected and the failing dimensions are returned

#### Scenario: Promotion of T5 asset requires SDLC Team
- **WHEN** an asset at trust level T5 is moved toward `approved`
- **THEN** the transition is held until the SDLC Team approves explicitly, the approval is audited, and only then the asset becomes `approved`

#### Scenario: Production flow rejects non-approved asset
- **WHEN** Alfred attempts to invoke an asset whose `lifecycle_state` is not `approved` in a production-relevant flow
- **THEN** the platform rejects the invocation and audits the attempt

#### Scenario: Deprecated asset is discoverable with warning
- **WHEN** a Workspace lists assets and includes a deprecated one
- **THEN** the asset is shown with a deprecation banner, a pointer to the recommended replacement, and a notice discouraging new adoption

## ADDED Requirements

### Requirement: Trust levels T0–T5 enforced
The Registry SHALL classify each asset with a trust level: **T0 Experimental, T1 Read-only, T2 Internal Write, T3 SDLC Write, T4 Infra/Deploy, T5 Critical/Core**. Trust level SHALL drive review depth, eval thresholds, allowed environments and required approvers.

#### Scenario: Trust-level change requires re-approval
- **WHEN** an owner increases the trust level of an asset
- **THEN** the asset returns to `in_review` and must be re-approved according to the new level's requirements

### Requirement: Eval scores attached to assets
Approved assets SHALL carry `eval_scores` covering quality, safety, cost and latency from the eval harness. Scores SHALL be visible in the Asset detail view.

#### Scenario: Eval scores visible in detail view
- **WHEN** a user opens an approved asset's detail view
- **THEN** the latest eval scores per dimension and trend over versions are displayed
