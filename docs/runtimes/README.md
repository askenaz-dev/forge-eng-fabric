# Runtimes

Forge supports BYO and Provisioned runtimes for `gke`, `cloudrun`, and `minikube`.

BYO runtime registration stores kubeconfig or service account credentials encrypted through KMS and requires preflight checks for connectivity, RBAC, namespace creation, registry access, and Workload Identity where applicable.

Provisioned runtimes are created by `POST /v1/runtimes/provision`, which requires a per-Tenant Terraform state backend and emits Terraform plan/apply audit events.
