variable "project_id" { type = string }
variable "tenant_id" { type = string }
variable "runtime_type" {
  type    = string
  default = "gke"
  validation {
    condition     = contains(["gke", "cloud-run", "minikube"], var.runtime_type)
    error_message = "runtime_type must be one of: gke, cloud-run, minikube"
  }
}
variable "forge_principal" { type = string }
