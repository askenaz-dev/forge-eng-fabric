# iam-delegated-permissions

Provisions the federated IAM scope minimization required by `forge-provisioned-runtime` and `byo-runtime-onboarding`. Creates per-Tenant SAs with the minimum roles Forge needs to deploy and observe — never `roles/owner`, never `roles/editor`.

## Required inputs

| Variable | Type | Description |
|---|---|---|
| `project_id` | string | The Tenant's GCP project ID |
| `tenant_id` | string | Forge Tenant ID; used as the SA naming prefix |
| `runtime_type` | string | `gke`, `cloud-run`, or `minikube` (drives which roles get bound) |
| `forge_principal` | string | The Forge platform SA that needs to impersonate the Tenant SA |

## Outputs

| Output | Description |
|---|---|
| `runtime_sa_email` | The SA created for runtime workloads |
| `deploy_sa_email` | The SA Forge uses to deploy via this Tenant project |
| `roles_granted` | The list of roles bound to each SA (for evidence capture) |

## Roles granted (per runtime_type)

| runtime_type | Roles on `runtime_sa_email` |
|---|---|
| `gke` | `roles/container.developer` (workload), `roles/artifactregistry.reader` |
| `cloud-run` | `roles/run.invoker`, `roles/artifactregistry.reader` |
| `minikube` | (none — local cluster, no GCP roles bound) |

`deploy_sa_email` always gets `roles/iam.serviceAccountTokenCreator` so Forge can mint short-lived tokens via Workload Identity Federation.

## Example

```hcl
module "forge_iam" {
  source           = "../../modules/iam-delegated-permissions"
  project_id       = var.project_id
  tenant_id        = "tenant-acme"
  runtime_type     = "gke"
  forge_principal  = "serviceAccount:forge-platform@forge-control-plane.iam.gserviceaccount.com"
}
```

## Verification

After apply, run `make verify-runtime` against the registered runtime — the `federated_iam_scope` check asserts the SA has the expected roles and nothing more (returns `fail` if `roles/owner` or `roles/editor` is detected).
