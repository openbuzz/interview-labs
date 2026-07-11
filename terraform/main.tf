module "aws" {
  source = "./aws"
  count  = var.cloud_provider == "aws" ? 1 : 0

  size           = var.size
  image          = var.image
  slug           = var.slug
  ssh_public_key = tls_private_key.ssh.public_key_openssh
}

module "digitalocean" {
  source = "./digitalocean"
  count  = var.cloud_provider == "digitalocean" ? 1 : 0

  region         = var.region
  size           = var.size
  image          = var.image
  slug           = var.slug
  ssh_public_key = tls_private_key.ssh.public_key_openssh
}

module "hetzner" {
  source = "./hetzner"
  count  = var.cloud_provider == "hetzner" ? 1 : 0

  region         = var.region
  size           = var.size
  image          = var.image
  slug           = var.slug
  ssh_public_key = tls_private_key.ssh.public_key_openssh
}

# The active VM module's address, shared by the ip output and the DNS record.
locals {
  vm_ip = coalesce(
    one(module.digitalocean[*].ip),
    one(module.hetzner[*].ip),
    one(module.aws[*].ip),
  )
}

module "cloudflare" {
  source = "./cloudflare"
  count  = var.dns_enabled ? 1 : 0

  zone_id = var.cloudflare_zone_id
  slug    = var.slug
  ip      = local.vm_ip
}

# Session keypair lives in the root module; provider modules consume the public key.
resource "tls_private_key" "ssh" {
  algorithm = "ED25519"
}

resource "local_sensitive_file" "ssh_private_key" {
  content         = tls_private_key.ssh.private_key_openssh
  filename        = "${var.ssh_dir}/key"
  file_permission = "0600"
}

resource "local_file" "ssh_public_key" {
  content         = tls_private_key.ssh.public_key_openssh
  filename        = "${var.ssh_dir}/key.pub"
  file_permission = "0644"
}
