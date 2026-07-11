# shellcheck shell=bash

# shellcheck source=tests/tui/lib/tmux.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/tmux.sh"

setup() {
  export XDG_CONFIG_HOME="${BATS_TEST_TMPDIR}/config"
  export XDG_STATE_HOME="${BATS_TEST_TMPDIR}/state"
  export XDG_CACHE_HOME="${BATS_TEST_TMPDIR}/cache"
  mkdir -p "${XDG_CONFIG_HOME}" "${XDG_STATE_HOME}" "${XDG_CACHE_HOME}"
}

teardown() {
  tmux_kill
}

# tui_cmd <invocation> — prefix the XDG env inline so the command sees this
# test's dirs regardless of the tmux server's environment.
tui_cmd() {
  printf 'XDG_CONFIG_HOME=%q XDG_STATE_HOME=%q XDG_CACHE_HOME=%q %s' \
    "${XDG_CONFIG_HOME}" "${XDG_STATE_HOME}" "${XDG_CACHE_HOME}" "$1"
}

# seed_session <slug> <ip> <status> <age-minutes> — write a schema-2 session.
seed_session() {
  local slug="$1"
  local ip="$2"
  local status="$3"
  local age_min="$4"
  local dir="${XDG_STATE_HOME}/interview/sessions/${slug}"
  mkdir -p "${dir}"

  local created
  created="$(date -u -d "-${age_min} minutes" +%Y-%m-%dT%H:%M:%SZ)"
  cat >"${dir}/metadata.json" <<EOF
{
  "schema": 2,
  "slug": "${slug}",
  "created_at": "${created}",
  "region": "fra1",
  "size": "s-2vcpu-2gb",
  "image": "ubuntu-26-04-x64",
  "roles": { "vm": "digitalocean" },
  "ssh_user": "root",
  "terraform": { "binary": "terraform", "version": "1.9.0" },
  "ip": "${ip}",
  "status": "${status}",
  "phase": "summary"
}
EOF
}
