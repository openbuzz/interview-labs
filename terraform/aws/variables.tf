variable "size" {
  description = "EC2 instance type."
  type        = string
}

variable "image" {
  description = "AMI name filter resolved against Canonical's images."
  type        = string
}

variable "slug" {
  description = "Session slug, embedded in resource names."
  type        = string
}

variable "ssh_public_key" {
  description = "OpenSSH public key authorized on the instance."
  type        = string
}
