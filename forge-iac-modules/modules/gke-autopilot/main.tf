variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "cluster_name" {
  type = string
}

resource "google_container_cluster" "autopilot" {
  name             = var.cluster_name
  project          = var.project_id
  location         = var.region
  enable_autopilot = true
}

output "cluster_name" {
  value = google_container_cluster.autopilot.name
}

output "endpoint" {
  value     = google_container_cluster.autopilot.endpoint
  sensitive = true
}
