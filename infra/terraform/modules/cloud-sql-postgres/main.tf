resource "google_sql_database_instance" "this" {
  project          = var.project_id
  name             = "${var.name}-postgres"
  region           = var.region
  database_version = "POSTGRES_16"

  settings {
    tier              = "db-custom-1-3840"
    availability_type = "ZONAL"
    disk_autoresize   = true

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
      transaction_log_retention_days = 7
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = var.network_id
    }
  }
}

resource "google_sql_database" "control_plane" {
  project  = var.project_id
  instance = google_sql_database_instance.this.name
  name     = "forge_control_plane"
}

resource "google_sql_database" "registry" {
  project  = var.project_id
  instance = google_sql_database_instance.this.name
  name     = "forge_registry"
}

resource "google_sql_database" "audit" {
  project  = var.project_id
  instance = google_sql_database_instance.this.name
  name     = "forge_audit"
}
