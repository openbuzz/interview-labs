output "ip" {
  description = "Public IPv4 address of the session VM."
  value       = one(module.digitalocean[*].ip)
}
