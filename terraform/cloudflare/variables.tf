variable "zone_id" {
  description = "Cloudflare zone that receives the session record."
  type        = string
}

variable "slug" {
  description = "Session slug; becomes the record name under the zone."
  type        = string
}

variable "ip" {
  description = "Public IPv4 address of the session VM."
  type        = string
}
