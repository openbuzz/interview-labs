# Resolves the image slug at plan time so a bad or retired image fails
# before any resource is created.
data "digitalocean_image" "this" {
  slug = var.image
}

resource "digitalocean_droplet" "this" {
  name     = "interview-labs-${var.slug}-vm"
  region   = var.region
  size     = var.size
  image    = data.digitalocean_image.this.slug
  ssh_keys = [digitalocean_ssh_key.this.fingerprint]
  tags     = ["interview-labs", "slug:${var.slug}"]
}

resource "digitalocean_ssh_key" "this" {
  name       = "interview-labs-${var.slug}-key"
  public_key = var.ssh_public_key
}

# Disposable interview VM: ssh open to the world by design (access is gated
# by the session keypair and pinned host key), full egress for tooling installs.
#trivy:ignore:DIG-0001
#trivy:ignore:DIG-0003
resource "digitalocean_firewall" "this" {
  name        = "interview-labs-${var.slug}-fw"
  droplet_ids = [digitalocean_droplet.this.id]

  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "icmp"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}
