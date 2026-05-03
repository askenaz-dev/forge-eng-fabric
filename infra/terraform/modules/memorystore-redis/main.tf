resource "google_redis_instance" "this" {
  project        = var.project_id
  name           = "${var.name}-redis"
  tier           = "BASIC"
  memory_size_gb = 1
  region         = var.region
  redis_version  = "REDIS_7_0"
  authorized_network = var.network_id
}
