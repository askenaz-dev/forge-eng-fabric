# Forge IaC Modules

Terraform modules used by Provisioned by Forge runtimes. Runtime provisioning composes these modules for GCP project boundaries, GKE Autopilot, Cloud Run, Artifact Registry access, and Workload Identity.

Each Tenant must bootstrap a GCS backend bucket with versioning, CMEK encryption, and Terraform state locking before provisioning runtimes.
