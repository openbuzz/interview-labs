# interview-labs

*Stop testing answers. Start testing work.*

`interview` deploys a disposable interview environment per session — a cloud VM on
DigitalOcean, Hetzner Cloud, or AWS, or containers on your own machine via the `local`
provider. Each session runs the interview stack: a password-gated web gateway, a
browser VS Code workspace (four profiles: `backend`, `devops`, `backend-ai`,
`devops-ai`), and an isolated docker daemon for the candidate. Cloud VMs install
docker via cloud-init and pull the stack images
(`ghcr.io/openbuzz/interview-labs-{gateway,vscode}`) at launch; the handover prints the
session URL and gateway password.
When configured, `-ai` sessions also get a spend-capped OpenRouter API key (minted at
launch, revoked at destroy) and every cloud session a proxied Cloudflare DNS record
(`<slug>.<your-domain>`).
Sessions run in parallel; state lives under your XDG directories and survives restarts.

## Requirements

- terraform or opentofu on PATH (terraform preferred when both exist)
- credentials for at least one cloud provider (DigitalOcean/Hetzner/AWS token), or
  just a running docker engine for local sessions
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
interview launch    # pick provider, profile, region and size; pulls and starts the stack
interview list      # sessions with age and status
interview info      # one session's details: IP, OS, ssh line
interview ssh       # shell into a session VM
# local sessions: "interview ssh" execs into the vscode container instead
interview destroy   # tear a session down
```

Pass `--no-ai` / `--no-dns` to skip the per-session AI key or DNS record when the
providers are configured. The proxied DNS URL serves nothing until the VM hosts a web
service (Cloudflare answers 522) — it is groundwork.

`--profile` picks the vscode stack (`backend`, `devops`, `backend-ai`, `devops-ai`;
non-interactive default `devops`). `-ai` profiles read the minted OpenRouter key from
the environment at start — the key value is never written to disk on either side.
Local sessions bind `http://localhost:8080` with the fixed password `openbuzz`; cloud
sessions serve `http://<ip>` (port 80) — or `http://<slug>.<domain>` with Cloudflare —
with a random per-session password (shown in the handover and `interview info`).

Images are pulled prebuilt from the registry; `--image` / `--gateway` override the
vscode / gateway ref outright (full ref, used verbatim), and `--tag` swaps in a local,
unqualified tag instead. Dev loop: `task docker:build` builds local images, then
`interview launch` (pick `local`) `--tag local` runs them without touching the registry.

Non-interactive use: set a provider env var (`DIGITALOCEAN_TOKEN`, `HCLOUD_TOKEN`, or
`AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY`; `OPENROUTER_MANAGEMENT_KEY` and
`CLOUDFLARE_API_TOKEN` override their config entries) and pass
`--region`/`--size`/`--profile` to launch, `--yes` to destroy.

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
  profile: devops-ai  # remembered stack-profile pick
  ```

- sessions: `$XDG_STATE_HOME/interview/sessions/<slug>/` — terraform state, ssh key,
  logs; archived metadata+logs land in `archive/<slug>/` after destroy
- provider cache: `$XDG_CACHE_HOME/interview/terraform/plugins/`

The session's terraform state contains the generated SSH private key; both stay local
with 0600 file modes.
