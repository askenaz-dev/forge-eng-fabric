variable "project_id" {
  type = string
}

variable "namespace" {
  type = string
}

variable "kubernetes_service_account" {
  type = string
}

variable "gcp_service_account_id" {
  type = string
}

resource "google_service_account" "deployer" {
  project      = var.project_id
  account_id   = var.gcp_service_account_id
  display_name = "Forge runtime deployer"
}

resource "google_service_account_iam_member" "workload_identity" {
  service_account_id = google_service_account.deployer.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[${var.namespace}/${var.kubernetes_service_account}]"
}

output "sa_email" {
  value = google_service_account.deployer.email
}
