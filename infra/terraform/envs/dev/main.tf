terraform {
  required_version = ">= 1.7.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.40"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

module "network" {
  source     = "../../modules/network"
  project_id = var.project_id
  region     = var.region
  name       = var.name
}

module "gke" {
  source     = "../../modules/gke-autopilot"
  project_id = var.project_id
  region     = var.region
  name       = var.name
  network    = module.network.network_self_link
  subnetwork = module.network.subnet_self_link
}

module "postgres" {
  source     = "../../modules/cloud-sql-postgres"
  project_id = var.project_id
  region     = var.region
  name       = var.name
  network_id = module.network.network_id
}

module "redis" {
  source     = "../../modules/memorystore-redis"
  project_id = var.project_id
  region     = var.region
  name       = var.name
  network_id = module.network.network_id
}

module "artifact_registry" {
  source     = "../../modules/artifact-registry"
  project_id = var.project_id
  region     = var.region
  name       = var.name
}
