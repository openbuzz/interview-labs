output "ip" {
  description = "Public IPv4 address of the session VM."
  value = coalesce(
    one(module.digitalocean[*].ip),
    one(module.hetzner[*].ip),
    one(module.aws[*].ip),
  )
}
