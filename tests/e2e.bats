#!/usr/bin/env bats

# Root end-to-end test. Boots the real gateway + vscode stack via the root compose
# file, then drives the live auth and proxy path against real code-server and ttyd.
# Mirrors images/gateway/tests/smoke.bats, but with no nginx simulator.

COMPOSE_CMD="docker compose -f ${BATS_TEST_DIRNAME}/../docker/compose.yaml"
BASE_URL="http://localhost:8080"
PASSWORD="${GATEWAY_PASSWORD:-openbuzz}"

setup_file() {
  if ! ${COMPOSE_CMD} up -d --build --wait >/dev/null; then
    echo "compose up failed" >&2
    ${COMPOSE_CMD} logs >&2 || true
    return 1
  fi

  # compose --wait already gated health above; this only polls boot-to-healthy,
  # never a build/pull. Keep the cap tight — 15 x 2s.
  local i
  for (( i = 0; i < 15; i++ )); do
    if curl -fsS "${BASE_URL}/healthz" >/dev/null; then
      return 0
    fi
    sleep 2
  done

  echo "stack not healthy within 30s" >&2
  ${COMPOSE_CMD} logs >&2 || true
  return 1
}

teardown_file() {
  ${COMPOSE_CMD} down -v -t 0 >/dev/null || true
}

@test "gateway /healthz is public" {
  run curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/healthz"
  [[ "${output}" == "200" ]]
}

@test "unauthenticated /ide/ redirects to /login" {
  run curl -s -o /dev/null -w '%{http_code} %{redirect_url}' "${BASE_URL}/ide/"
  [[ "${output}" == 302* ]]
  [[ "${output}" == *"/login"* ]]
}

@test "wrong password is rejected and sets no cookie" {
  local jar="${BATS_TEST_TMPDIR}/wrong.jar"
  run curl -s -o /dev/null -w '%{http_code}' -c "${jar}" \
    --data 'password=definitely-wrong' "${BASE_URL}/login"
  [[ "${output}" == "401" ]]
  run grep -c gw_auth "${jar}"
  [[ "${output}" == "0" ]]
}

@test "correct password authenticates and proxies to real code-server" {
  local jar="${BATS_TEST_TMPDIR}/ok.jar"
  curl -s -o /dev/null -c "${jar}" --data "password=${PASSWORD}" "${BASE_URL}/login"

  # code-server's own /healthz, reached through the gateway with the /ide prefix stripped.
  run curl -s -o /dev/null -w '%{http_code}' -b "${jar}" "${BASE_URL}/ide/healthz"
  [[ "${output}" == "200" ]]
}

@test "authenticated /term/ reaches real ttyd" {
  local jar="${BATS_TEST_TMPDIR}/term.jar"
  curl -s -o /dev/null -c "${jar}" --data "password=${PASSWORD}" "${BASE_URL}/login"
  run curl -s -o /dev/null -w '%{http_code}' -b "${jar}" "${BASE_URL}/term/"
  [[ "${output}" == "200" ]]
}

@test "authenticated WebSocket upgrade proxies through to ttyd" {
  local jar="${BATS_TEST_TMPDIR}/ws-term.jar"
  curl -s -o /dev/null -c "${jar}" --data "password=${PASSWORD}" "${BASE_URL}/login"
  run curl -s -o /dev/null -w '%{http_code}' -b "${jar}" \
    -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
    -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
    "${BASE_URL}/term/ws"
  [[ "${output}" == "101" ]]
}

@test "forged cookie is rejected" {
  run curl -s -o /dev/null -w '%{http_code}' \
    --cookie 'gw_auth=9999999999.deadbeef' "${BASE_URL}/ide/"
  [[ "${output}" == "302" ]]
}

@test "vscode docker client runs a container against the dind daemon" {
  run ${COMPOSE_CMD} exec -T vscode docker run --rm hello-world
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"Hello from Docker!"* ]]
}
