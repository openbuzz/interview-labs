# Session keypair lives in the root module; provider modules consume the public key.
resource "tls_private_key" "session" {
  algorithm = "ED25519"
}

module "digitalocean" {
  source = "./digitalocean"

  region         = var.region
  size           = var.size
  image          = var.image
  slug           = var.slug
  ssh_public_key = tls_private_key.session.public_key_openssh
}
