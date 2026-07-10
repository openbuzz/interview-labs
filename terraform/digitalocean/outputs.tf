output "ip" {
  description = "Public IPv4 address of the droplet."
  value       = digitalocean_droplet.this.ipv4_address
}
