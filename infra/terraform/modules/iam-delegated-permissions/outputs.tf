output "runtime_sa_email" {
  value = google_service_account.runtime_sa.email
}

output "deploy_sa_email" {
  value = google_service_account.deploy_sa.email
}

output "roles_granted" {
  value = local.runtime_roles[var.runtime_type]
}
