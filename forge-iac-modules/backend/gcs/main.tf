variable "tenant_id" {
  type = string
}

variable "location" {
  type    = string
  default = "US"
}

variable "kms_key_name" {
  type = string
}

resource "google_storage_bucket" "terraform_state" {
  name                        = "forge-tfstate-${var.tenant_id}"
  location                    = var.location
  uniform_bucket_level_access = true
  force_destroy               = false

  versioning {
    enabled = true
  }

  encryption {
    default_kms_key_name = var.kms_key_name
  }
}

output "bucket_name" {
  value = google_storage_bucket.terraform_state.name
}
