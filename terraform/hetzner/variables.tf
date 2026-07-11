variable "region" {
  description = "Hetzner Cloud location slug."
  type        = string
}

variable "size" {
  description = "Server type slug."
  type        = string
}

variable "image" {
  description = "Server image slug."
  type        = string
}

variable "slug" {
  description = "Session slug, embedded in resource names."
  type        = string
}

variable "ssh_public_key" {
  description = "OpenSSH public key authorized on the server."
  type        = string
}
