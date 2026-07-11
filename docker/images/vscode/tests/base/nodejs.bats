#!/usr/bin/env bats
# shellcheck disable=SC2016  # single-quoted strings expand inside the container shell

load ../setup_suite

@test "node is on the global PATH (non-interactive shell)" {
  run in_container bash -c 'command -v node'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "/usr/local/bin/node" ]]
}

@test "node resolves to /opt/nodejs (isolated runtime)" {
  run in_container bash -c 'readlink -f "$(command -v node)"'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "/opt/nodejs/bin/node" ]]
}

@test "node version matches the Dockerfile" {
  local want
  want="$(dockerfile_arg NODE_VERSION)"
  [[ -n "${want}" ]]

  run in_container node --version
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "v${want}" ]]
}

@test "npm reports a version" {
  run in_container npm --version
  [[ "${status}" -eq 0 ]]
  [[ -n "${output}" ]]
}

@test "npx reports a version" {
  run in_container npx --version
  [[ "${status}" -eq 0 ]]
  [[ -n "${output}" ]]
}

@test "node executes a script" {
  run in_container node -e 'console.log("it works")'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "it works" ]]
}

@test "npm bash completion is installed and loads" {
  run in_container test -s /etc/bash_completion.d/npm
  [[ "${status}" -eq 0 ]]

  run in_container bash -ic 'complete -p npm'
  [[ "${status}" -eq 0 ]]
}

@test "npm global bin dir is on PATH (load-bearing for global CLIs)" {
  run in_container bash -c 'echo "${PATH}"'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"/opt/nodejs/bin"* ]]
}
