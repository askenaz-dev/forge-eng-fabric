# artifact-registry

Provisions an Artifact Registry repository scoped to a Forge Tenant or Workspace, with retention and IAM bindings for the runtime SA.

## Required inputs

| Variable | Type | Description |
|---|---|---|
| `project_id` | string | GCP project ID |
| `location` | string | Region (default `us-central1`) |
| `repo_id` | string | Repository ID (e.g., `forge-platform`, `forge-<workspace>`) |
| `format` | string | Repository format (default `DOCKER`) |
| `runtime_sa_email` | string | The runtime SA that needs `roles/artifactregistry.reader` |

## Outputs

| Output | Description |
|---|---|
| `repository_url` | The fully-qualified repo URL (e.g., `us-central1-docker.pkg.dev/<project>/<repo>`) |
| `iam_binding` | The created IAM binding name |

## Retention

Production repos enable an immutable retention policy of 30 days (configurable). The retention is part of the [data-retention policy](../../../../docs/governance/data-retention.md).

## Example

```hcl
module "forge_artifact_registry" {
  source           = "../../modules/artifact-registry"
  project_id       = var.project_id
  location         = "us-central1"
  repo_id          = "forge-platform"
  format           = "DOCKER"
  runtime_sa_email = module.forge_iam.runtime_sa_email
}
```

## Cosign verification

Images pushed to this repo MUST be signed via Cosign keyless. See [`infra/helm/README.md`](../../../helm/README.md) for the verification command and [Phase 2 sign-off](../../../../docs/governance/phase-2-signoff.md) for evidence requirements.
