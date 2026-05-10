## ADDED Requirements

### Requirement: Helm chart per platform service

The platform SHALL ship a Helm chart at `infra/helm/<service>/` for every service in `services/` that is intended to run in Kubernetes. Each chart SHALL be installable independently of the umbrella chart.

#### Scenario: Chart exists for every Kubernetes-bound service

- **WHEN** a release is built
- **THEN** the build SHALL fail if any service in `services/` declared `kubernetes: true` in its manifest lacks a chart at `infra/helm/<service>/`

#### Scenario: Independent install of a single chart

- **WHEN** an operator runs `helm install <service> infra/helm/<service>` with valid values
- **THEN** the chart SHALL render successfully without requiring the umbrella chart

### Requirement: Three flavor templates

Every service chart SHALL be derived from one of three flavor templates: `service-http`, `service-worker`, or `service-cron`, declared in `Chart.yaml` annotations. Deviations SHALL be documented in the chart README.

#### Scenario: Flavor declared in Chart.yaml

- **WHEN** a contributor opens any chart
- **THEN** `Chart.yaml` SHALL include `annotations.forge.platform/flavor: service-http|service-worker|service-cron`

### Requirement: Required Kubernetes resources per chart

Every chart SHALL produce: `Deployment` (or `CronJob` for cron flavor), `Service` (HTTP flavor only), `ServiceMonitor`, `NetworkPolicy`, `HorizontalPodAutoscaler` (HTTP and worker flavors), `PodDisruptionBudget` (HTTP and worker flavors), `ConfigMap`/`Secret` references via projected volumes when applicable, and `ServiceAccount` with the minimum-required RBAC.

#### Scenario: Chart linting verifies required resources

- **WHEN** `make helm-lint` runs
- **THEN** every chart SHALL pass a lint check that verifies the presence of required resources for its declared flavor

#### Scenario: NetworkPolicy enforces deny-by-default

- **WHEN** any service chart renders its NetworkPolicy
- **THEN** the policy SHALL be deny-by-default with explicit egress rules to declared dependencies and explicit ingress rules from declared callers

### Requirement: Per-environment values files

Each chart SHALL include `values.yaml` (defaults) plus environment overlays at `values-local.yaml`, `values-staging.yaml`, `values-prod.yaml`. The overlays SHALL set image tags, replica counts, resource requests/limits, and observability configuration appropriate to the environment.

#### Scenario: Environment overlay applied

- **WHEN** an operator installs with `-f values-staging.yaml`
- **THEN** replica counts, requests, and limits SHALL match the Staging row of the platform sizing document

### Requirement: Umbrella chart `forge-platform`

The platform SHALL ship `infra/helm/forge-platform/` as an umbrella chart that depends on every service chart and exposes tier presets (`small`, `medium`, `large`) plus opinionated `values-local.yaml`, `values-staging.yaml`, `values-prod.yaml` files.

#### Scenario: Umbrella install brings up the platform

- **WHEN** an operator runs `helm install forge-platform infra/helm/forge-platform -f values-staging.yaml`
- **THEN** the install SHALL render every dependent service's resources with the staging tier values and pass `helm template` validation

#### Scenario: Tier preset selects sizing

- **WHEN** the operator passes `--set tier=medium`
- **THEN** every dependent service's `requests`, `limits`, and `replicaCount` SHALL match the medium-tier values from the sizing document

### Requirement: Charts versioned and signed

Every chart and the umbrella SHALL declare a SemVer-compliant `version`, MUST be packaged on release, and the resulting chart artifacts SHALL be signed with Cosign.

#### Scenario: Release publishes signed charts

- **WHEN** a release publishes charts to the chart repository
- **THEN** each `.tgz` artifact SHALL have an accompanying signature artifact verifiable with Cosign and the platform's public key

### Requirement: Chart README and example values

Every chart SHALL include `README.md` documenting purpose, required values, optional values, dependencies, and a copy-paste example install command.

#### Scenario: README is current with values

- **WHEN** a chart's `values.yaml` adds a new top-level key
- **THEN** the chart's `README.md` SHALL be updated in the same change to document the new key
