variable "region" {
  type        = string
  description = "DigitalOcean region slug."
}

variable "size" {
  type        = string
  description = "Droplet size slug."
}

variable "image" {
  type        = string
  description = "Droplet image slug."
}

variable "slug" {
  type        = string
  description = "Session slug, embedded in resource names."
}

variable "ssh_public_key" {
  type        = string
  description = "OpenSSH public key authorized on the droplet."
}
