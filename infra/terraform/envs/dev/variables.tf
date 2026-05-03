variable "project_id" {
  type        = string
  description = "GCP project for the Phase 0 bootstrap environment."
}

variable "region" {
  type        = string
  description = "GCP region."
  default     = "us-central1"
}

variable "name" {
  type        = string
  description = "Environment name prefix."
  default     = "forge-dev"
}
