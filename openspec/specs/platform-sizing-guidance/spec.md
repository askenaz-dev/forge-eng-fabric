# platform-sizing-guidance Specification

## Purpose
TBD - created by archiving change platform-gaps-closure. Update Purpose after archive.
## Requirements

### Requirement: Documented sizing tiers for Local, Staging, Production

The platform documentation SHALL include a "Hardware & Sizing" section in `docs/platform-enablement.md` describing three tiers: Local (single developer laptop), Staging (shared team environment), and Production (per-BU or per-Workspace cluster). Each tier SHALL list minimum and recommended RAM, vCPU, disk, and network requirements covering the full set of platform components in scope for that tier.

#### Scenario: Local tier specifies minimum and recommended

- **WHEN** a developer reads the Local tier section
- **THEN** the document SHALL specify minimum RAM (when running the full compose stack) and recommended RAM, with an explicit list of compose services that may be disabled to reduce footprint and the disable instructions

#### Scenario: Staging tier specifies per-service requests/limits

- **WHEN** a platform operator reads the Staging tier section
- **THEN** the document SHALL list per-service Kubernetes `requests` and `limits` for CPU and memory, plus minimum replica counts, sufficient to render values files for the Helm umbrella chart

#### Scenario: Production tier specifies per-BU dimensioning

- **WHEN** a platform operator reads the Production tier section
- **THEN** the document SHALL specify GKE node pool sizes, Cloud SQL tiers, Memorystore tiers, Milvus dimensions, and Kafka/Loki/Tempo retention sizing for at least three BU profiles (small ≤10 apps, medium ≤50 apps, large ≤200 apps)

### Requirement: Cost estimate per tier

Each tier SHALL include an indicative monthly cost estimate, the assumptions behind the estimate (region, currency, list-price baseline), and the date of last refresh.

#### Scenario: Cost estimate cites assumptions

- **WHEN** a stakeholder reads any tier's cost estimate
- **THEN** the estimate SHALL state its currency, the cloud region used (e.g., `us-central1`), the SKUs assumed, the date of last refresh, and the volatility caveat

### Requirement: Sizing values surfaced in Helm chart values

The Helm umbrella chart SHALL expose tier presets (`tier=small`, `tier=medium`, `tier=large`) that map to the Staging and Production sizing in this document.

#### Scenario: Operator installs medium tier

- **WHEN** an operator runs `helm install forge-platform infra/helm/forge-platform -f values-medium.yaml`
- **THEN** the resulting deployment SHALL match the dimensioning specified in the medium-tier row of the sizing document for every service

### Requirement: Sizing changes are reviewed alongside platform releases

Changes to the sizing document SHALL be tracked in the change history and SHALL be reviewed by Platform Architecture and FinOps owners.

#### Scenario: Sizing document updated for a release

- **WHEN** a service's resource profile changes materially in a release
- **THEN** the corresponding sizing-tier rows SHALL be updated in the same change as the resource update, with the prior values preserved in the Markdown change log
