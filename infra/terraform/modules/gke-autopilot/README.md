# gke-autopilot

Provisions a GKE Autopilot cluster wired up with Workload Identity for Forge runtimes.

This module is the **`gke-cluster`** entry referenced in the [Phase 3 enablement](../../../../docs/platform-enablement.md#phase-3-deployable-apps) and the [`runtime-connectors`](../../../../openspec/specs/runtime-connectors/spec.md) spec.

## Required inputs

| Variable | Type | Description |
|---|---|---|
| `project_id` | string | GCP project ID hosting the cluster |
| `region` | string | GCP region (e.g., `us-central1`) |
| `name` | string | Cluster base name; the resource is `${name}-gke` |
| `network` | string | VPC name or self-link |
| `subnetwork` | string | Subnet name or self-link with secondary ranges named `pods` and `services` |

## Outputs

| Output | Description |
|---|---|
| `cluster_name` | The resulting GKE cluster name |
| `location` | The cluster's region or zone |

## Example

```hcl
module "forge_runtime" {
  source     = "../../modules/gke-autopilot"
  project_id = var.project_id
  region     = "us-central1"
  name       = "forge-pilot"
  network    = google_compute_network.vpc.self_link
  subnetwork = google_compute_subnetwork.subnet.self_link
}
```

## Notes

- Workload Identity is enabled by default with pool `${project_id}.svc.id.goog` so platform SAs can be bound to KSAs.
- `release_channel = REGULAR` is set; flip to `RAPID` only with explicit Platform Architecture review.
- The IP allocation policy assumes secondary ranges named `pods` and `services` exist on the subnetwork; create those alongside the network module.

## Related

- [`iam-delegated-permissions`](../iam-delegated-permissions/README.md) — federated SA bindings
- [`artifact-registry`](../artifact-registry/README.md) — image-pull bindings
- [Phase 3 sign-off](../../../../docs/governance/phase-3-signoff.md)
