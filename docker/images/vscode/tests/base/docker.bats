#!/usr/bin/env bats

load ../setup_suite

@test "docker client is on the global PATH" {
  run in_container bash -c 'command -v docker'
  [[ "${status}" -eq 0 ]]
}

@test "docker reports a client version" {
  run in_container docker --version
  [[ "${status}" -eq 0 ]]
  [[ -n "${output}" ]]
}

@test "the buildx plugin is available" {
  run in_container docker buildx version
  [[ "${status}" -eq 0 ]]
}

@test "the compose plugin is available" {
  run in_container docker compose version
  [[ "${status}" -eq 0 ]]
}

@test "docker bash completion is installed and loads" {
  local f=/usr/share/bash-completion/completions/docker
  run in_container bash -ic "source ${f} && complete -p docker"
  [[ "${status}" -eq 0 ]]
}
