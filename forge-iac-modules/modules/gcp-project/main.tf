variable "tenant_id" {
  type = string
}

variable "workspace_id" {
  type = string
}

variable "env" {
  type = string
}

variable "billing_account" {
  type = string
}

variable "folder_id" {
  type = string
}

resource "google_project" "workspace" {
  name            = "forge-${var.workspace_id}-${var.env}"
  project_id      = "forge-${var.workspace_id}-${var.env}"
  folder_id       = var.folder_id
  billing_account = var.billing_account

  labels = {
    tenant    = var.tenant_id
    workspace = var.workspace_id
    env       = var.env
    managed_by = "forge"
  }
}

output "project_id" {
  value = google_project.workspace.project_id
}
