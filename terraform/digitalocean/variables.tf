variable "region" {
  description = "DigitalOcean region slug."
  type        = string
}

variable "size" {
  description = "Droplet size slug."
  type        = string
}

variable "image" {
  description = "Droplet image slug."
  type        = string
}

variable "slug" {
  description = "Session slug, embedded in resource names."
  type        = string
}

variable "ssh_public_key" {
  description = "OpenSSH public key authorized on the droplet."
  type        = string
}
