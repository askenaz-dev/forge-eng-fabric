resource "google_artifact_registry_repository" "this" {
  project       = var.project_id
  location      = var.region
  repository_id = replace("${var.name}-containers", "_", "-")
  description   = "Forge Phase 0 container images"
  format        = "DOCKER"
}
