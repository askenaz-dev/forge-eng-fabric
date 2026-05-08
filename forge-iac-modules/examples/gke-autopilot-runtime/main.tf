module "project" {
  source          = "../../modules/gcp-project"
  tenant_id       = var.tenant_id
  workspace_id    = var.workspace_id
  env             = var.env
  billing_account = var.billing_account
  folder_id       = var.folder_id
}

module "gke" {
  source       = "../../modules/gke-autopilot"
  project_id   = module.project.project_id
  region       = var.region
  cluster_name = "forge-${var.workspace_id}-${var.env}"
}

module "workload_identity" {
  source                     = "../../modules/workload-identity"
  project_id                 = module.project.project_id
  namespace                  = "forge-apps"
  kubernetes_service_account = "forge-deployer"
  gcp_service_account_id     = "forge-deployer"
}

output "project_id" {
  value = module.project.project_id
}

output "cluster_name" {
  value = module.gke.cluster_name
}

output "endpoint" {
  value     = module.gke.endpoint
  sensitive = true
}

output "sa_email" {
  value = module.workload_identity.sa_email
}
