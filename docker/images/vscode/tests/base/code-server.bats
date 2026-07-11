#!/usr/bin/env bats

load ../setup_suite

@test "code-server version matches the Dockerfile" {
  local want
  want="$(dockerfile_arg CODE_SERVER_VERSION)"
  [[ -n "${want}" ]]

  run in_container code-server --version
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"${want}"* ]]
}

@test "code-server is listening and healthy on port 8080" {
  run in_container curl -fsS http://127.0.0.1:8080/healthz
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *'"status"'* ]]
}

@test "container reports healthy (Docker HEALTHCHECK)" {
  run docker inspect -f '{{.State.Health.Status}}' "${CONTAINER}"
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "healthy" ]]
}

@test "baked settings.json matches the source file (sha256)" {
  local src="${BATS_TEST_DIRNAME}/../../layers/base/files/user/settings.json"
  local baked="/home/user/.local/share/code-server/User/settings.json"
  local want got

  want="$(docker exec -i "${CONTAINER}" sha256sum <"${src}" | cut -d' ' -f1)"
  got="$(in_container sha256sum "${baked}" | cut -d' ' -f1)"
  [[ -n "${want}" ]]
  [[ "${got}" == "${want}" ]]
}
