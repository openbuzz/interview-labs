variable "region" {
  type        = string
  description = "Hetzner Cloud location slug."
}

variable "size" {
  type        = string
  description = "Server type slug."
}

variable "image" {
  type        = string
  description = "Server image slug."
}

variable "slug" {
  type        = string
  description = "Session slug, embedded in resource names."
}

variable "ssh_public_key" {
  type        = string
  description = "OpenSSH public key authorized on the server."
}
