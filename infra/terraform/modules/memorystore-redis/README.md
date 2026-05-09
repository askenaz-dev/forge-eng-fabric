# memorystore-redis

Provisions a Memorystore Redis instance for Forge platform caches and rate-limit counters.

Equivalent to the **`memorystore`** module entry in the Phase 3 enablement guide.

## Required inputs

| Variable | Type | Description |
|---|---|---|
| `project_id` | string | GCP project ID |
| `region` | string | Region for the instance |
| `name` | string | Instance base name |
| `tier` | string | `BASIC` (Staging) or `STANDARD_HA` (Production) |
| `memory_size_gb` | number | Capacity in GB; see sizing table |
| `network` | string | VPC self-link for private IP |

## Outputs

| Output | Description |
|---|---|
| `host` | Redis host (private IP) |
| `port` | Redis port (default 6379) |

## Sizing reference

| BU profile | Memory | Tier |
|---|---:|---|
| Small | 5 GiB | `STANDARD_HA` |
| Medium | 16 GiB | `STANDARD_HA` |
| Large | 32 GiB | `STANDARD_HA` |
| Staging | 1 GiB | `BASIC` |

See [Hardware & Sizing](../../../../docs/platform-enablement.md#hardware--sizing).

## Example

```hcl
module "forge_redis" {
  source         = "../../modules/memorystore-redis"
  project_id     = var.project_id
  region         = "us-central1"
  name           = "forge-pilot"
  tier           = "STANDARD_HA"
  memory_size_gb = 5
  network        = google_compute_network.vpc.self_link
}
```
