variable "tenant_id" {
  type = string
}

variable "workspace_id" {
  type = string
}

variable "env" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "billing_account" {
  type = string
}

variable "folder_id" {
  type = string
}
