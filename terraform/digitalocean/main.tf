resource "digitalocean_ssh_key" "session" {
  name       = "il-${var.slug}-key"
  public_key = var.ssh_public_key
}

resource "digitalocean_droplet" "session" {
  name     = "il-${var.slug}-vm"
  region   = var.region
  size     = var.size
  image    = var.image
  ssh_keys = [digitalocean_ssh_key.session.fingerprint]
  tags     = ["interview-labs", "slug:${var.slug}"]
}

resource "digitalocean_firewall" "session" {
  name        = "il-${var.slug}-fw"
  droplet_ids = [digitalocean_droplet.session.id]

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
