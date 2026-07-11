# Inactive providers still get configured at plan time: hcloud rejects
# tokens that are not exactly 64 characters, and aws validates credentials
# against STS even with count=0 resources. Placeholders keep the unselected
# providers plannable; active providers read credentials from the child env
# (HCLOUD_TOKEN, AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY, CLOUDFLARE_API_TOKEN).

provider "aws" {
  region = var.cloud_provider == "aws" ? var.region : "us-east-1"

  skip_credentials_validation = var.cloud_provider != "aws"
  skip_requesting_account_id  = var.cloud_provider != "aws"
  access_key                  = var.cloud_provider == "aws" ? null : "placeholder"
  secret_key                  = var.cloud_provider == "aws" ? null : "placeholder"
}

provider "hcloud" {
  token = (var.cloud_provider == "hetzner" ? null :
  "0000000000000000000000000000000000000000000000000000000000000000")
}

# The cloudflare provider validates the token format even with count=0
# resources; the placeholder keeps DNS-less sessions plannable. When DNS is
# on, api_token = null makes it read CLOUDFLARE_API_TOKEN from the child env.
provider "cloudflare" {
  api_token = (var.dns_enabled ? null :
  "0000000000000000000000000000000000000000")
}
