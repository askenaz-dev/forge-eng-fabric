# repo-template-catalog Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Templates as governed assets

Repo templates SHALL be registered in the Registry as assets of `type=repo_template`, versioned by SemVer, with explicit `lifecycle_state` and `trust_level`.

#### Scenario: Only approved templates are usable in production

- **GIVEN** a template `python-fastapi-microservice@1.0.0` with `lifecycle_state=approved` and `trust_level=T3`
- **WHEN** an onboarding request references it
- **THEN** the catalog MUST return the template manifest
- **AND** the onboarding service MUST proceed

#### Scenario: Reject in-review templates from production onboarding

- **GIVEN** a template in `lifecycle_state=in_review`
- **WHEN** a non-pilot Workspace tries to use it
- **THEN** the catalog MUST refuse with `403 template_not_approved`

### Requirement: Template manifest schema

Each template MUST declare a `template.yaml` containing: `id`, `version`, `description`, `parameters` (with type and validations), `pre_hooks`, `post_hooks`, `files`, `required_capabilities`.

#### Scenario: Reject template with invalid manifest

- **GIVEN** a template tag whose `template.yaml` fails schema validation
- **WHEN** registration is attempted
- **THEN** the catalog MUST refuse the registration and emit `repo_template.registration.rejected.v1`

### Requirement: Versioning and immutability

Template versions SHALL be immutable once published; corrections require a new SemVer version. Tags MUST be signed.

#### Scenario: Reject overwrite of existing version

- **GIVEN** template `go-microservice@1.0.0` already registered
- **WHEN** a publish attempts to overwrite the same version
- **THEN** the catalog MUST refuse with `409 version_already_exists`

### Requirement: Discovery API

The catalog SHALL expose `GET /v1/templates` and `GET /v1/templates/{id}/versions` with filters by `lifecycle_state`, `trust_level`, `category`, and Workspace visibility.

#### Scenario: List approved templates for a Workspace

- **GIVEN** a user authenticated to Workspace `ws-1`
- **WHEN** they call `GET /v1/templates?lifecycle_state=approved`
- **THEN** the catalog MUST return only templates visible to `ws-1`
