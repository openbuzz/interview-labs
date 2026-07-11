#!/usr/bin/env bash

IMAGE="${IMAGE:-interview-labs-vscode:base-local}"
CONTAINER="interview-labs-vscode-suite"

# Run a command inside the running suite container.
in_container() {
  docker exec "${CONTAINER}" "$@"
}

# Echo the value of an ARG (e.g. CODE_SERVER_VERSION=4.125.0 -> 4.125.0) from the Dockerfile.
dockerfile_arg() {
  local name="${1}"
  local dockerfile="${BATS_TEST_DIRNAME}/../../layers/base/Dockerfile"
  local line

  line="$(grep -oE "${name}=[0-9][0-9.]*" "${dockerfile}" | head -n1)"
  echo "${line#"${name}="}"
}

# Echo the version string a profile's mise manifest pins for a tool.
# Handles bare keys (go = "1.24") and aqua keys ("aqua:hashicorp/terraform" = "1.10").
# Usage: mise_tool_version devops terraform
mise_tool_version() {
  local profile="${1}" tool="${2}" manifest
  manifest="${BATS_TEST_DIRNAME}/../../layers/${profile}/mise.toml"
  grep -E "(^|/)${tool}\"?[[:space:]]*=" "${manifest}" |
    grep -oE '"[0-9][^"]*"' | tr -d '"' | head -n1
}

setup_suite() {
  docker rm -f "${CONTAINER}" >/dev/null || true
  docker run -d --name "${CONTAINER}" "${IMAGE}" >/dev/null

  # Image built by the `build` dep; docker run above only started it. 15 x 2s.
  local i status="" tpl='{{if .State.Health}}{{.State.Health.Status}}{{end}}'
  for ((i = 0; i < 15; i++)); do
    status="$(docker inspect -f "${tpl}" "${CONTAINER}" || true)"
    [[ "${status}" == "healthy" ]] && return 0
    sleep 2
  done

  echo "container not healthy within 30s (status: ${status})" >&2
  docker logs "${CONTAINER}" >&2 || true
  return 1
}

teardown_suite() {
  docker rm -f "${CONTAINER}" >/dev/null || true
}
