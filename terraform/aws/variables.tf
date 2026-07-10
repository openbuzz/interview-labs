variable "size" {
  type        = string
  description = "EC2 instance type."
}

variable "image" {
  type        = string
  description = "AMI name filter resolved against Canonical's images."
}

variable "slug" {
  type        = string
  description = "Session slug, embedded in resource names."
}

variable "ssh_public_key" {
  type        = string
  description = "OpenSSH public key authorized on the instance."
}
