variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "service_name" {
  type = string
}

variable "image" {
  type = string
}

resource "google_cloud_run_v2_service" "service" {
  name     = var.service_name
  project  = var.project_id
  location = var.region

  template {
    containers {
      image = var.image
    }
  }
}

output "service_name" {
  value = google_cloud_run_v2_service.service.name
}

output "uri" {
  value = google_cloud_run_v2_service.service.uri
}
