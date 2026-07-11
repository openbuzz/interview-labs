#!/usr/bin/env bats

load ../setup_suite

@test "mise is on the global PATH" {
  run in_container bash -c 'command -v mise'
  [[ "${status}" -eq 0 ]]
}

@test "mise reports its version" {
  run in_container mise --version
  [[ "${status}" -eq 0 ]]
}

@test "the mise shim dir is on PATH for non-interactive shells" {
  run in_container bash -c 'echo "${PATH}"'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"/opt/mise/shims"* ]]
}

@test "mise bash completion is installed and loads" {
  run in_container test -s /etc/bash_completion.d/mise
  [[ "${status}" -eq 0 ]]

  run in_container bash -ic 'complete -p mise'
  [[ "${status}" -eq 0 ]]
}

@test "jq is on the global PATH" {
  run in_container bash -c 'command -v jq'
  [[ "${status}" -eq 0 ]]
}
