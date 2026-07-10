resource "hcloud_ssh_key" "this" {
  name       = "interview-labs-${var.slug}-key"
  public_key = var.ssh_public_key
}

resource "hcloud_server" "this" {
  name        = "interview-labs-${var.slug}-vm"
  location    = var.region
  server_type = var.size
  image       = var.image
  ssh_keys    = [hcloud_ssh_key.this.id]
  labels = {
    interview-labs = "true"
    slug           = var.slug
  }
}

# Hetzner firewalls default-deny inbound once attached and default-allow
# outbound when no outbound rules exist.
resource "hcloud_firewall" "this" {
  name = "interview-labs-${var.slug}-fw"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  apply_to {
    server = hcloud_server.this.id
  }
}
