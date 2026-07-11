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

variable "dns_enabled" {
  description = "Whether the session gets a Cloudflare DNS record."
  type        = bool
  default     = false
}

variable "cloudflare_zone_id" {
  description = "Cloudflare zone that receives the session record."
  type        = string
  default     = ""
}

variable "dns_base_domain" {
  description = "Zone apex domain, used to spell out the fqdn output."
  type        = string
  default     = ""
}
