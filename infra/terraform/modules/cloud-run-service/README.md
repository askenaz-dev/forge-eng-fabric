# cloud-run-service

Provisions a Cloud Run service for Forge runtime targets that prefer serverless over GKE.

This is the **`cloud-run-service`** module entry referenced in the [Phase 3 enablement](../../../../docs/platform-enablement.md#phase-3-deployable-apps) guide.

## Required inputs

| Variable | Type | Description |
|---|---|---|
| `project_id` | string | GCP project ID |
| `region` | string | Region (e.g., `us-central1`) |
| `name` | string | Service name |
| `image` | string | Container image (signed digest required for production) |
| `service_account_email` | string | SA the service runs as |
| `ingress` | string | `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER` (default), `INGRESS_TRAFFIC_INTERNAL_ONLY`, or `INGRESS_TRAFFIC_ALL` |
| `concurrency` | number | Max concurrent requests per instance (default 80) |
| `min_instances` | number | Cold-start mitigation; 0 for cost optimization |
| `max_instances` | number | Upper bound for autoscaling |

## Outputs

| Output | Description |
|---|---|
| `service_url` | The Cloud Run service URL |
| `service_name` | The Cloud Run service name |

## Example

```hcl
module "forge_cloud_run_app" {
  source                = "../../modules/cloud-run-service"
  project_id            = var.project_id
  region                = "us-central1"
  name                  = "forge-app-pilot"
  image                 = "us-central1-docker.pkg.dev/${var.project_id}/forge-platform/app@sha256:..."
  service_account_email = module.forge_iam.deploy_sa_email
  ingress               = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
}
```

## Image signing

Production deploys MUST use a Cosign-signed digest. The `services/deploy-orchestrator` verifier blocks unsigned digests at runtime.
