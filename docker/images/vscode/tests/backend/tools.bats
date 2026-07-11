#!/usr/bin/env bats

load ../setup_suite

@test "go is on the global PATH" {
  run in_container bash -c 'command -v go'
  [[ "${status}" -eq 0 ]]
}

@test "go version matches the backend manifest" {
  local want
  want="$(mise_tool_version backend go)"
  [[ -n "${want}" ]]

  run in_container go version
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"${want}"* ]]
}

@test "the C toolchain is present for native builds" {
  run in_container bash -c 'command -v gcc && command -v make'
  [[ "${status}" -eq 0 ]]
}

@test "go editor tools are on the global PATH" {
  run in_container bash -c 'command -v gopls && command -v dlv && command -v staticcheck'
  [[ "${status}" -eq 0 ]]
}

@test "JS package managers and typescript are on PATH" {
  run in_container bash -c 'command -v pnpm && command -v yarn && command -v tsc'
  [[ "${status}" -eq 0 ]]
}

@test "tsc compiles a trivial file" {
  run in_container bash -c \
    'cd /tmp && echo "const x: number = 1; console.log(x);" > t.ts && tsc t.ts && node t.js'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "1" ]]
}
