output "ip" {
  value = module.digitalocean.ip
}

output "ssh_private_key" {
  value     = tls_private_key.session.private_key_openssh
  sensitive = true
}

output "ssh_public_key" {
  value = tls_private_key.session.public_key_openssh
}
