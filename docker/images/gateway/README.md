# interview-labs-gateway

A small Go (standard-library-only) auth proxy that password-gates the `vscode` image. After a
successful login it sets a signed, time-limited cookie; only requests bearing a valid cookie are
proxied to code-server and ttyd.

## What it does

- `GET /login` — password form. `POST /login` — checks the password, sets the `gw_auth` cookie.
- `/ide/...` — prefix-stripped, proxied to code-server (`GATEWAY_IDE_UPSTREAM`).
- `/term/...` — prefix-stripped, proxied to ttyd (`GATEWAY_TERM_UPSTREAM`); ttyd must run with
  `--base-path /term` (the vscode image's `TTYD_BASE_PATH=/term`).
- `GET /healthz` — unauthenticated liveness probe.
- `GET /styles.css` — public stylesheet shared by the login page and the landing page. The pages
  themselves are plain HTML.
- WebSocket upgrades pass through transparently.

## Configuration

Precedence is **flag > environment variable > built-in default**.

| CLI flag | Env var | Default | Purpose |
|----------|---------|---------|---------|
| `-addr` | `GATEWAY_ADDR` | `:8080` | Listen address |
| `-password` | `GATEWAY_PASSWORD` | *(required)* | Shared login password; empty → refuse to start |
| `-secret` | `GATEWAY_SECRET` | *(random)* | 256-bit cookie-signing key; pin to persist sessions / run replicas |
| `-secret-file` | `GATEWAY_SECRET_FILE` | *(unset)* | Persist the auto-generated secret here (survives restart) |
| `-ttl-minutes` | `GATEWAY_TTL_MINUTES` | `120` | Cookie lifetime (minutes) |
| `-ide-upstream` | `GATEWAY_IDE_UPSTREAM` | `http://vscode:8080` | code-server upstream |
| `-term-upstream` | `GATEWAY_TERM_UPSTREAM` | `http://vscode:7681` | ttyd upstream |
| `-secure-cookie` | `GATEWAY_SECURE_COOKIE` | `false` | Set `Secure` on the auth cookie (behind TLS) |
| `-login-burst` | `GATEWAY_LOGIN_BURST` | `5` | Allowed `POST /login` per IP per window |
| `-login-window-seconds` | `GATEWAY_LOGIN_WINDOW_SECONDS` | `30` | Rate-limit window (seconds) |
| `-trust-proxy` | `GATEWAY_TRUST_PROXY` | `false` | Trust `X-Forwarded-For` (only behind a trusted proxy) |
| `-log-level` | `GATEWAY_LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `-log-format` | `GATEWAY_LOG_FORMAT` | `text` | `text` or `json` |
| `-tls-cert` | `GATEWAY_TLS_CERT` | *(unset)* | TLS certificate path; set with `-tls-key` |
| `-tls-key` | `GATEWAY_TLS_KEY` | *(unset)* | TLS private key path; set with `-tls-cert` |
| `-landing-page` | `GATEWAY_LANDING_PAGE` | *(unset)* | Content fragment injected into the landing shell at `/`; unset → built-in default stub |
| `-healthcheck` | *(—)* | `false` | Probe `/healthz` on the local listener and exit (in-container probe) |

## Usage

| Command | What it does |
|---------|--------------|
| `task check` | Verify required host tools |
| `task build` | Build `interview-labs-gateway:local` |
| `task test` | Go unit tests + compose smoke test |
| `task lint` | hadolint + gofmt + go vet |
| `task run` | Run gateway + backend simulator on `:8080` |
| `task down` | Stop and clear the running stack |
| `task scan` | trivy source scan (gating) + image scan (informational) |

## Health check

Docker `HEALTHCHECK` only runs a command — on a `scratch`-based image there is no `curl` or
`wget`. The binary self-probes instead: `/gateway -healthcheck` makes a single HTTP request to
`/healthz` on the configured listen address and exits 0 on 200, non-zero otherwise. External
orchestrators (Kubernetes liveness/readiness probes, ECS health checks) may also probe `/healthz`
directly over HTTP (or HTTPS when TLS is enabled).

The bundled Docker `HEALTHCHECK` runs `/gateway -healthcheck` with no flags, so its TLS
detection is env-driven — set `GATEWAY_TLS_CERT` / `GATEWAY_TLS_KEY` via env if you rely on the
in-container probe over https.

## Landing page

`/` always serves a landing page after login. The gateway owns the page **shell** (document
skeleton, `<title>`, and the shared stylesheet link); you supply only the **content fragment** —
plain, primitive markup (headings, paragraphs, `<section>` blocks), no `<html>`/`<head>`/`<body>`
and no styles. All styling comes from `/styles.css`.

When `GATEWAY_LANDING_PAGE` (or `-landing-page`) is unset, a built-in **default stub** fragment is
used, so the page works out of the box. Point the flag/env at your own fragment file (mounted, or
baked into a derived image) to customize it; the gateway wraps it in the shell and serves it.
Edits to a mounted fragment take effect on the next request — no restart needed. A configured but
unreadable path is a boot error; if the file disappears at runtime the gateway logs it and falls
back to the default stub. The fragment is injected as trusted HTML (operator-authored, not user
input). Non-`/` authenticated paths redirect to `/ide/`. See `tests/landing.html` for a sample
fragment.

## TLS

Set **both** `GATEWAY_TLS_CERT` and `GATEWAY_TLS_KEY` to a certificate/key pair to enable TLS.
The gateway listens on the same address (`GATEWAY_ADDR`, default `:8080`) and switches to HTTPS.
Setting exactly one of the two is a boot error.

> **Do not stack TLS.** TLS belongs in exactly one place on the path. If a load balancer or
> reverse proxy in front of the gateway already terminates TLS (the common production setup),
> leave `GATEWAY_TLS_CERT` / `GATEWAY_TLS_KEY` **unset** — the gateway serves plain HTTP behind
> the LB. Set them **only** when the gateway is itself the TLS edge (no terminating proxy in
> front). Enabling gateway TLS *and* terminating at the LB means either double encryption or a
> protocol mismatch, and gains nothing.
>
> `GATEWAY_SECURE_COOKIE` is separate: behind a TLS-terminating LB the gateway speaks plain HTTP
> but the browser still uses HTTPS, so set `GATEWAY_SECURE_COOKIE=true` there even though gateway
> TLS is off.

## Security

- Cookie: `HMAC-SHA256(secret, exp)`, constant-time verified, `HttpOnly`, `SameSite=Lax`. The
  signing secret is random and **not** derived from the password — a leaked cookie reveals
  nothing about the password and cannot be forged (2^256).
- Login is rate-limited per IP (5/30 s by default). Set `GATEWAY_TRUST_PROXY=true` only behind a
  trusted edge proxy/LB; otherwise `X-Forwarded-For` is ignored (it is spoofable).
- The password, signing secret, and cookie values are never logged.
- Logout and session revocation are out of scope — handled by restart (rotates the
  auto-generated secret), **unless** `GATEWAY_SECRET` or `GATEWAY_SECRET_FILE` pins it.

## Host prerequisites

`docker`, `go`, `go-task`, `hadolint`, `bats` (bats-core), `trivy`, `editorconfig-checker`,
`openssl` and `curl` (the last two used by the smoke test). Run `task check` to verify.
