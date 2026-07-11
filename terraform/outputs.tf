output "ip" {
  description = "Public IPv4 address of the session VM."
  value       = local.vm_ip
}

output "fqdn" {
  description = "Session DNS name behind the Cloudflare proxy; empty when DNS is off."
  value       = var.dns_enabled ? "${var.slug}.${var.dns_base_domain}" : ""
}
