resource "digitalocean_ssh_key" "this" {
  name       = "interview-labs-${var.slug}-key"
  public_key = var.ssh_public_key
}

resource "digitalocean_droplet" "this" {
  name     = "interview-labs-${var.slug}-vm"
  region   = var.region
  size     = var.size
  image    = var.image
  ssh_keys = [digitalocean_ssh_key.this.fingerprint]
  tags     = ["interview-labs", "slug:${var.slug}"]
}

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
