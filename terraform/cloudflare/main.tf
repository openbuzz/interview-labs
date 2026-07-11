# Proxied record: edge TLS and the VM's IP hidden behind Cloudflare. TTL 1
# is Cloudflare's required "automatic" value for proxied records. Serves
# nothing until the VM hosts a web service — groundwork by design.
resource "cloudflare_dns_record" "this" {
  zone_id = var.zone_id
  name    = var.slug
  type    = "A"
  content = var.ip
  proxied = true
  ttl     = 1
}
