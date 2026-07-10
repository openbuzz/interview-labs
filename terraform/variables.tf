variable "cloud_provider" {
  type        = string
  description = "Provider serving the vm role."

  validation {
    condition     = contains(["digitalocean"], var.cloud_provider)
    error_message = "Unsupported vm provider."
  }
}

variable "region" {
  type        = string
  description = "Provider region slug for the session VM."
}

variable "size" {
  type        = string
  description = "Provider instance size slug."
}

variable "image" {
  type        = string
  description = "OS image the session VM boots."
}

variable "slug" {
  type        = string
  description = "Session slug, embedded in cloud resource names."
}

variable "ssh_dir" {
  type        = string
  description = "Absolute path that receives the generated keypair (key, key.pub)."
}
