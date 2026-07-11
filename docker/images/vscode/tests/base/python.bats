#!/usr/bin/env bats
# shellcheck disable=SC2016  # single-quoted strings expand inside the container shell

load ../setup_suite

@test "uv version matches the Dockerfile" {
  local want
  want="$(dockerfile_arg UV_VERSION)"
  [[ -n "${want}" ]]

  run in_container uv --version
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"${want}"* ]]
}

@test "python3 is on the global PATH (non-interactive shell)" {
  run in_container bash -c 'command -v python3'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "/usr/local/bin/python3" ]]
}

@test "python (bare) is on the global PATH (non-interactive shell)" {
  run in_container bash -c 'command -v python'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "/usr/local/bin/python" ]]
}

@test "python and python3 resolve to the same interpreter" {
  run in_container bash -c '
    p="$(readlink -f "$(command -v python)")"
    q="$(readlink -f "$(command -v python3)")"
    [[ -n "${p}" && "${p}" == "${q}" ]]
  '
  [[ "${status}" -eq 0 ]]
}

@test "python3 resolves into /opt/python (isolated runtime)" {
  run in_container bash -c 'readlink -f "$(command -v python3)"'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == /opt/python/* ]]
}

@test "python version matches the Dockerfile" {
  local want
  want="$(dockerfile_arg PYTHON_VERSION)"
  [[ -n "${want}" ]]

  run in_container python3 --version
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"${want}"* ]]
}

@test "python executes a script" {
  run in_container python3 -c 'print("it works")'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "it works" ]]
}

@test "python stdlib modules import (standalone build is complete)" {
  run in_container python3 -c 'import ssl, sqlite3, lzma, ctypes, zlib, bz2'
  [[ "${status}" -eq 0 ]]
}

@test "uv bash completion is installed and loads" {
  run in_container test -s /etc/bash_completion.d/uv
  [[ "${status}" -eq 0 ]]

  run in_container bash -ic 'complete -p uv'
  [[ "${status}" -eq 0 ]]
}
