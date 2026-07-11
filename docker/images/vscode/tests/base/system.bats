#!/usr/bin/env bats
# shellcheck disable=SC2016  # single-quoted strings expand inside the container shell

load ../setup_suite

@test "runs as non-root user 'user' (uid/gid 1000)" {
  run in_container bash -c 'printf "%s:%s:%s" "$(whoami)" "$(id -u)" "$(id -g)"'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "user:1000:1000" ]]
}

@test "passwordless sudo works" {
  run in_container sudo -n true
  [[ "${status}" -eq 0 ]]
}

@test "workspace directory exists and is owned by user" {
  run in_container stat -c '%U:%G:%F' /home/user/workspace
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "user:user:directory" ]]
}

@test "bashrc.local is sourced and sets EDITOR + locale" {
  run in_container bash -ic 'echo "${EDITOR}:${LANG}"'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"vim:en_US.UTF-8"* ]]
}

@test "bash-completion plumbing loads in an interactive shell" {
  run in_container bash -ic '[[ -n "${BASH_COMPLETION_VERSINFO:-}" ]] && echo loaded'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"loaded"* ]]
}

@test "base CLI tools are available" {
  run in_container bash -c \
    'for cmd in curl find git htop mc nano ps tree unzip vim wget
    do command -v "${cmd}" || exit 1; done'
  [[ "${status}" -eq 0 ]]
}
