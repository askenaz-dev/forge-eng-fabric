# Spec Delta: ai-asset-registry-minimal (MODIFIED)

## MODIFIED Requirements

### Requirement: Application asset deployment sub-resource

The `application` asset type SHALL be extended with a sub-resource `deployment` representing release history per environment; entries MUST be immutable.

#### Scenario: Deployment recorded on asset

- **GIVEN** asset `application/app-foo`
- **WHEN** a deploy completes successfully to `env=stage`
- **THEN** a `deployment` entry MUST appear in the asset linking `revision_id`, `image_digest`, `runtime_id`, `env`, `verified_status`, `openspec_ids`, `pr_sha`, `created_at`, `actor`
- **AND** subsequent UPDATE/DELETE on the entry MUST be rejected by DB triggers

### Requirement: Deployment history query

`GET /v1/assets/{id}/deployments?env=<env>&limit=&cursor=` MUST return paginated history ordered by `created_at desc`.

#### Scenario: Paginate history for an env

- **GIVEN** asset with 50 prod deployments
- **WHEN** querying with `limit=20`
- **THEN** the response MUST return 20 entries plus a `next_cursor`
- **AND** subsequent cursor request MUST return the next page

### Requirement: Verified status flags exposed on asset

The asset response MUST expose `image.signature_verified` and `image.attestation_verified` flags for the latest deploy per env.

#### Scenario: Auditor checks signature status

- **GIVEN** asset `application/app-foo` with a verified deploy in `env=prod`
- **WHEN** the asset is queried
- **THEN** `deployments.prod.latest.image.signature_verified` MUST be `true`
- **AND** `deployments.prod.latest.image.attestation_verified` MUST be `true`
