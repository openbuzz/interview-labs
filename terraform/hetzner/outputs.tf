output "ip" {
  description = "Public IPv4 address of the server."
  value       = hcloud_server.this.ipv4_address
}
