#!/usr/bin/env bats

load ../setup_suite

@test "ttyd version matches the Dockerfile" {
  local want
  want="$(dockerfile_arg TTYD_VERSION)"
  [[ -n "${want}" ]]

  run in_container ttyd --version
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"${want}"* ]]
}

@test "ttyd serves on port 7681 (unauthenticated)" {
  run in_container curl -fsS http://127.0.0.1:7681
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"ttyd"* ]]
}
