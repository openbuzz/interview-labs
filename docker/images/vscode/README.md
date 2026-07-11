# interview-labs-vscode

Browser-based VS Code (code-server) image for interview-labs, published as four
stacks: `backend`, `devops`, `backend-ai`, and `devops-ai`.

## AI variants

`backend-ai` and `devops-ai` layer the AI assistant CLIs (claude-code, codex,
opencode) and their editor extensions on top of `backend` / `devops`. The
non-`-ai` stacks stay AI-free so AI-free interviews remain possible.

The CLIs route through OpenRouter. The launcher injects two variables (same value —
an OpenRouter key) at `docker run` / compose: `OPENROUTER_API_KEY` for codex and
opencode, and `ANTHROPIC_AUTH_TOKEN` for Claude Code, which has no provider config
and authenticates only via `ANTHROPIC_*` variables (the image bakes
`ANTHROPIC_BASE_URL` and `ANTHROPIC_MODEL`; see the root `compose.yaml` for the
pattern). No key is baked into the image.

Published stacks: `backend`, `devops`, `backend-ai`, `devops-ai`.

## Layers and stacks

A **layer** is a directory under `layers/` with its own `FROM parent` Dockerfile:
`base`, `backend`, `devops`, `ai`. `base` is `FROM ubuntu:26.04` and carries the
shared foundation (code-server, ttyd, Python/uv, Node, mise, Docker CLI, the
non-root user, base extensions). Non-base layers know nothing about their
neighbours — each just builds on whatever `parent` bake hands it.

A **stack** (a published image) is `base` plus an ordered set of layers, tagged
in canonical order `[backend, devops, ai]` with `base` implicit in every tag:

```
backend ──▶ backend-ai
devops  ──▶ devops-ai
```

Composition is declared in `docker-bake.hcl`: each target names its build
context (`layers/<name>`) and its parent via `contexts = { parent =
"target:<parent>" }`. `task build` (`docker buildx bake`) resolves that
parent-edge graph and builds it — there's no hand-written build order. `base`
is built as the shared parent and covered by its own tests, but it is an
**internal substrate, not a published image**: the published set is `backend`,
`devops`, `backend-ai`, `devops-ai`.

Each non-base layer directory holds what it needs:

- `layers/<name>/mise.toml` — declarative tool fleet; installed by `mise install` into
  `/opt/mise` and served via mise's shims (`/opt/mise/shims` is already on `PATH`).
- `layers/<name>/setup.sh` — imperative setup (apt packages, completions, post-install
  wiring). **Rule:** non-trivial build logic (loops, conditionals, multi-tool wiring)
  lives in a `setup.sh` because the Taskfile lints `layers/*/setup.sh` with
  shellcheck/shfmt; short, linear setup (≲10 lines) inlines in the Dockerfile via
  `RUN <<EOF` for locality — inline heredocs are **not** linted. Today only `devops`
  warrants a `setup.sh`; `backend` and `ai` inline.
- `layers/<name>/settings.json` — per-layer code-server settings delta, deep-merged
  onto the base `files/user/settings.json`.
- `layers/base/files/` holds the shared entrypoint, the code-server extension/settings
  helper commands (`files/bin/`, installed to `/usr/local/bin`), and default user
  settings/shell config baked into every stack.
- `ai` has no `mise.toml`/`settings.json`; it ships only `config/` (per-tool CLI config
  baked into `/home/user`, credentials excluded).

**Install lanes** used across the base image and non-base layers:

| Lane | Tools |
|------|-------|
| apt | system packages (bash-completion, ca-certificates, curl, git, jq, mc, nano, sudo, vim, …) |
| checksum-pinned binary | code-server, ttyd, uv, Node.js, mise — each downloaded and verified against a SHA256 `ARG` |
| vendor apt | Docker CLI + buildx + compose via Docker's signed apt repo (GPG fingerprint-verified) |
| mise / aqua (layer) | Go + gopls/dlv/staticcheck (backend); terraform, opentofu, kubectl, helm, aws-cli, k9s (devops) |

Tests live in `tests/{base,backend,devops,ai}/` and run against the matching
layer's image; `tests/ai/` runs once per `-ai` stack with `AI_PARENT` (an env var
passed to `bats`, not a build arg) selecting the parent-specific assertions.

**Adding a layer:** create `layers/<name>/` with its `FROM parent` Dockerfile
(plus `mise.toml` / `settings.json` / `setup.sh` as needed), then add its
target(s) and parent edge to `docker-bake.hcl`. CI derives its build/publish
targets from bake — there's no separate loop to edit.

## What's in it

Every stack shares the base image:

- `ubuntu:26.04` + a set of CLI utilities (curl, git, vim, nano, mc, less, …).
- code-server `4.125.0` (pinned via `ARG CODE_SERVER_VERSION`), served on port `8080`.
- ttyd `1.7.7` (pinned via `ARG TTYD_VERSION`), a browser terminal served on port `7681`.
- Python `3.14.6` and `uv` `0.11.24` (pinned via ARGs), on the global PATH; `uv` is the
  package and virtual-env manager.
- Node.js `24.18.0` LTS (pinned via `ARG NODE_VERSION`), with `npm`/`npx`.
- mise `2026.6.14` (pinned via `ARG MISE_VERSION`), runtime version manager; shims on
  the global `PATH`.
- Docker CLI + buildx + compose plugins (from Docker's signed apt repo) — **client only**; the
  daemon runs as a separate `dind` service in `compose.yaml`, reached over `DOCKER_HOST`.
- Bash completions for installed tools (`uv`, `npm`, `mise`, …) under
  `/etc/bash_completion.d`.
- A non-root `user` (uid/gid 1000) with passwordless sudo.
- The Material Icon Theme, plus base editor extensions: YAML (redhat),
  Python (ms-python), Prettier, Ruff (Python lint/format), and Container Tools.
- Default workspace at `/home/user/workspace`.
- A container `HEALTHCHECK` probing code-server's `/healthz` endpoint.

Layer additions on top of base:

- **`backend`**: Go `1.26` (via mise) with `gopls`, `dlv`, `staticcheck`;
  `build-essential` (gcc, make, headers; via apt); JS/TS tooling — `pnpm`,
  `yarn` (corepack), `typescript`.
- **`devops`**: terraform `1.15`, opentofu `1.12`, kubectl `1.36`, helm `4.2`,
  aws-cli `2`, k9s `0.51`, plus `tflint`, `kustomize`, `trivy`, and `ansible`
  (all via aqua/uv); bash completions for each tool.

## Bumping a pinned tool

Each download (code-server, ttyd, uv, Node, mise) is verified at build time against a SHA256
**pinned in the Dockerfile** (`*_SHA256_AMD64` / `*_SHA256_ARM64`), not a checksum fetched
from the same release — so a tampered release asset fails the build. When you bump a version,
update both arch hashes:

1. (Recommended) verify the upstream artifact once, out of band — uv:
   `gh attestation verify <asset> --repo astral-sh/uv`; Node: `gpgv` against the
   [`nodejs/release-keys`](https://github.com/nodejs/release-keys) keyring.
2. Record the digest into the matching ARG:
   - uv: `curl -fsSL .../uv-<arch>-unknown-linux-gnu.tar.gz.sha256`
   - Node: from `https://nodejs.org/dist/v<VER>/SHASUMS256.txt`
   - ttyd: from the release `SHA256SUMS`
   - mise: from the release `SHA256SUMS`
   - code-server: `sha256sum` of the downloaded `.deb` (no upstream checksums file)
3. Rebuild: a wrong hash fails at `sha256sum -c` — that is the gate working, not a flake.

Layer tools are version-locked in `layers/<name>/mise.toml`; bump them by editing that file.
Aqua-managed tools (the devops fleet) are additionally checksum-pinned by the aqua registry.

## Usage

All commands run from this directory (`images/vscode/`), or prefix with `vscode:`
from the repo root (e.g. `task vscode:build`).

| Command | What it does |
|---------|--------------|
| `task check` | Verify required host tools are installed |
| `task build` | Build every stack via `docker buildx bake` (targets and parent edges come from `docker-bake.hcl`) |
| `task run` | Run one stack locally (default `devops`; `PROFILE=backend\|devops\|backend-ai\|devops-ai`); IDE at http://localhost:8080, terminal at http://localhost:7681 |
| `task lint` | hadolint (`layers/*/Dockerfile`) + shellcheck & shfmt (`entrypoint.sh`, `files/bin/*`, and `layers/*/setup.sh`) |
| `task test` | Build every stack, then run each layer's bats suite (`tests/<name>/`) |
| `task scan` | trivy: source scan (gates) + image scan of every built stack (informational) |

`build` is the single source of the `docker buildx bake` command; `run`, `test`,
and `scan-image` depend on it (`deps: [build]`) and rebuild only when
`docker-bake.hcl` or `layers/**` change.

The image scan is **informational** this iteration: it reports HIGH/CRITICAL findings — mostly
CVEs vendored inside code-server (its node/Go dependencies), which can't be patched at the
Dockerfile level — but does not fail. The **source scan gates** on actionable findings (Dockerfile
misconfiguration, committed secrets). Image-scan gating is deferred to when CI is added.

## Terminal

Alongside the IDE, the image runs [ttyd](https://github.com/tsl0922/ttyd) — a lightweight
browser terminal — on port `7681`. It is a fallback for when the code-server terminal feels
slow: ttyd talks straight to a `bash` PTY instead of routing through the editor. The terminal
opens in the workspace (`/home/user/workspace`).

## Auth

This image is **unauthenticated by design**. code-server runs with `--auth none` and ttyd is
always open. Authentication is enforced by the separate **gateway** image
(`images/gateway/`), which password-gates access and proxies `/ide` to code-server and `/term`
to ttyd. Do not expose ports `8080`/`7681` directly to an untrusted network — run the image
behind the gateway, or keep it on localhost/a trusted network.

### ttyd sub-path

`TTYD_BASE_PATH` (default empty) sets ttyd's base path for sub-path hosting. Empty serves ttyd
at root (standalone `task run`). The gateway compose sets `TTYD_BASE_PATH=/term`.

## Host prerequisites

`go-task` (runs these tasks), plus `docker`, `hadolint`, `shellcheck`, `shfmt`,
`bats` (bats-core), `trivy`, and `jq`. Run `task check` to verify the latter set is present.
