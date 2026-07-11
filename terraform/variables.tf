variable "cloud_provider" {
  description = "Provider serving the vm role."
  type        = string

  validation {
    condition     = contains(["digitalocean", "hetzner", "aws"], var.cloud_provider)
    error_message = "Unsupported vm provider."
  }
}

variable "region" {
  description = "Provider region slug for the session VM."
  type        = string
}

variable "size" {
  description = "Provider instance size slug."
  type        = string
}

variable "image" {
  description = "OS image the session VM boots."
  type        = string
}

variable "slug" {
  description = "Session slug, embedded in cloud resource names."
  type        = string
}

variable "ssh_dir" {
  description = "Absolute path that receives the generated keypair (key, key.pub)."
  type        = string
}
