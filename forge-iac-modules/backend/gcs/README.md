# Tenant GCS Backend

Bootstrap one backend bucket per Tenant and configure Terraform with:

```hcl
terraform {
  backend "gcs" {
    bucket = "forge-tfstate-<tenant>"
    prefix = "runtimes/<workspace>/<env>"
  }
}
```

The bucket must enable object versioning, CMEK encryption, uniform bucket-level access, and retention appropriate for audit requirements.
