locals {
  runtime_roles = {
    "gke" = [
      "roles/container.developer",
      "roles/artifactregistry.reader",
    ]
    "cloud-run" = [
      "roles/run.invoker",
      "roles/artifactregistry.reader",
    ]
    "minikube" = []
  }
}

resource "google_service_account" "runtime_sa" {
  project      = var.project_id
  account_id   = "forge-${var.tenant_id}-rt"
  display_name = "Forge runtime SA for tenant ${var.tenant_id}"
}

resource "google_service_account" "deploy_sa" {
  project      = var.project_id
  account_id   = "forge-${var.tenant_id}-dep"
  display_name = "Forge deploy SA for tenant ${var.tenant_id}"
}

resource "google_project_iam_member" "runtime_roles" {
  for_each = toset(local.runtime_roles[var.runtime_type])
  project  = var.project_id
  role     = each.value
  member   = "serviceAccount:${google_service_account.runtime_sa.email}"
}

resource "google_service_account_iam_member" "forge_can_impersonate_deploy" {
  service_account_id = google_service_account.deploy_sa.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = var.forge_principal
}
