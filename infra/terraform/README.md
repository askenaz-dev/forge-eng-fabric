# Forge Phase 0 Terraform

This Terraform layout provisions the GCP substrate required by `phase-0-foundations`: VPC, GKE Autopilot, Cloud SQL PostgreSQL, Memorystore Redis and Artifact Registry. Values are intentionally small for a development/bootstrap environment.

Terraform is not vendored in this repo. Run validation/apply from a workstation or CI runner with Terraform and GCP credentials configured.
