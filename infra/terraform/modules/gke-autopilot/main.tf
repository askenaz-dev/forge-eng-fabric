resource "google_container_cluster" "this" {
  project          = var.project_id
  name             = "${var.name}-gke"
  location         = var.region
  network          = var.network
  subnetwork       = var.subnetwork
  enable_autopilot = true

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  release_channel { channel = "REGULAR" }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }
}
