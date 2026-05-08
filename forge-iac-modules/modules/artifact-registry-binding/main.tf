variable "project_id" {
  type = string
}

variable "repository" {
  type = string
}

variable "member" {
  type = string
}

resource "google_artifact_registry_repository_iam_member" "reader" {
  project    = var.project_id
  location   = split("/", var.repository)[3]
  repository = split("/", var.repository)[5]
  role       = "roles/artifactregistry.reader"
  member     = var.member
}

output "binding_role" {
  value = google_artifact_registry_repository_iam_member.reader.role
}
