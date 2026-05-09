# cloud-sql-postgres

Provisions a Cloud SQL Postgres instance with private IP, CMEK, and the connection string output expected by Forge platform services.

Equivalent to the **`cloud-sql`** module entry in the Phase 3 enablement guide.

## Required inputs

| Variable | Type | Description |
|---|---|---|
| `project_id` | string | GCP project ID |
| `region` | string | Region for the instance |
| `name` | string | Instance base name |
| `tier` | string | Cloud SQL machine tier (e.g., `db-custom-2-7680` for staging, `db-custom-8-30720` for medium-tier prod) |
| `network` | string | VPC self-link for private IP |
| `kms_key` | string | CMEK key resource name |

## Outputs

| Output | Description |
|---|---|
| `instance_name` | Cloud SQL instance name |
| `connection_name` | `<project>:<region>:<name>` connection string for the SQL proxy |
| `private_ip` | Private IP for VPC-attached clients |

## Sizing reference

See the [Hardware & Sizing table](../../../../docs/platform-enablement.md#hardware--sizing) for tier mappings; the umbrella chart's `values-{staging,prod}.yaml` reference the matching `tier` for each BU profile.

## Example

```hcl
module "forge_postgres" {
  source     = "../../modules/cloud-sql-postgres"
  project_id = var.project_id
  region     = "us-central1"
  name       = "forge-pilot-pg"
  tier       = "db-custom-2-7680"
  network    = google_compute_network.vpc.self_link
  kms_key    = google_kms_crypto_key.platform.id
}
```

## Notes

- Backups are enabled with point-in-time recovery; retention defaults to 7 days for staging and 30 days for prod (override per Tenant policy).
- The `tier` change procedure: update via Terraform plan, then run `make sizing-check` to confirm Helm values still match the Hardware & Sizing table.
