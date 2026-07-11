# interview-labs

*Stop testing answers. Start testing work.*

`interview` deploys a disposable cloud VM per interview session — DigitalOcean, Hetzner
Cloud, or AWS — connects over SSH, and tears it down when you are done.
When configured, each session also gets a spend-capped OpenRouter API key (minted at
launch, revoked at destroy) and a proxied Cloudflare DNS record (`<slug>.<your-domain>`).
Sessions run in parallel; state lives under your XDG directories and survives restarts.

## Requirements

- terraform or opentofu on PATH (terraform preferred when both exist)
- credentials for at least one provider: a DigitalOcean API token, a Hetzner Cloud API
  token, or AWS IAM user credentials
- optional: an ssh client for `interview ssh`
- optional: an OpenRouter management key (mints per-session API keys) and a Cloudflare
  API token + zone for per-session DNS

## Install

```sh
go install github.com/openbuzz/interview-labs/cmd/interview@latest
```

## Use

```sh
interview doctor    # check tools, dirs, credentials
interview init      # configure cloud providers
interview launch    # pick region and size, deploy, mint AI key/DNS, prints Hello world from the VM
interview list      # sessions with age and status
interview info      # one session's details: IP, OS, ssh line
interview ssh       # shell into a session VM
interview destroy   # tear a session down
```

Pass `--no-ai` / `--no-dns` to skip the per-session AI key or DNS record when the
providers are configured. The proxied DNS URL serves nothing until the VM hosts a web
service (Cloudflare answers 522) — it is groundwork.

Non-interactive use: set a provider env var (`DIGITALOCEAN_TOKEN`, `HCLOUD_TOKEN`, or
`AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY`; `OPENROUTER_MANAGEMENT_KEY` and
`CLOUDFLARE_API_TOKEN` override their config entries) and pass `--region`/`--size` to
launch, `--yes` to destroy.

## State

- config: `$XDG_CONFIG_HOME/interview/config.yaml` (0600):

  ```yaml
  providers:
    digitalocean:
      token: "dop_v1_..."
      region: fra1
      instance: s-1vcpu-1gb
    hetzner:
      token: "..."
      region: fsn1
      instance: cx22
    aws:
      access_key_id: "AKIA..."
      secret_access_key: "..."
      region: eu-central-1
      instance: m7i.xlarge
    openrouter:
      management_key: "sk-or-..."
      cap_usd: 10
    cloudflare:
      api_token: "..."
      zone_id: "..."
      domain: example.com

  roles:
    vm: hetzner
  ```

- sessions: `$XDG_STATE_HOME/interview/sessions/<slug>/` — terraform state, ssh key,
  logs; archived metadata+logs land in `archive/<slug>/` after destroy
- provider cache: `$XDG_CACHE_HOME/interview/terraform/plugins/`

The session's terraform state contains the generated SSH private key; both stay local
with 0600 file modes.
