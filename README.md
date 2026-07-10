# interview-labs

`interview` deploys a disposable DigitalOcean VM per interview session, connects over
SSH, and tears it down when you are done. Sessions run in parallel; state lives under
your XDG directories and survives restarts.

## Requirements

- terraform or opentofu on PATH (terraform preferred when both exist)
- a DigitalOcean API token
- optional: an ssh client for `interview ssh`

## Install

```sh
go install github.com/openbuzz/interview-labs/cmd/interview@latest
```

## Use

```sh
interview doctor    # check tools, dirs, credentials
interview init      # store and validate the DigitalOcean token
interview launch    # pick region and size, deploy, prints Hello world from the VM
interview list      # sessions with age and status
interview ssh       # shell into a session VM
interview destroy   # tear a session down
```

Non-interactive use: set `DIGITALOCEAN_TOKEN` and pass `--region`/`--size` to launch,
`--yes` to destroy.

## State

- config: `$XDG_CONFIG_HOME/interview/config.yaml` (0600)
- sessions: `$XDG_STATE_HOME/interview/sessions/<slug>/` — terraform state, ssh key,
  logs; archived metadata+logs land in `archive/<slug>/` after destroy
- provider cache: `$XDG_CACHE_HOME/interview/terraform/plugins/`

The session's terraform state contains the generated SSH private key; both stay local
with 0600 file modes.
