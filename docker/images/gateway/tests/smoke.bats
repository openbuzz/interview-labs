#!/usr/bin/env bats

# Host-side end-to-end smoke test. Brings up the gateway + nginx simulator via compose,
# exercises auth, prefix stripping, rate limiting, and log redaction, then tears down.

COMPOSE_CMD="docker compose -f ${BATS_TEST_DIRNAME}/compose.yaml"
BASE_URL="http://localhost:8080"
PASSWORD="openbuzz"

setup_file() {
  if ! ${COMPOSE_CMD} up -d --build >/dev/null; then
    echo "compose up failed" >&2
    ${COMPOSE_CMD} logs >&2 || true
    return 1
  fi

  # Build already ran above; this polls boot-to-healthy only. 15 x 2s.
  local i
  for (( i = 0; i < 15; i++ )); do
    if curl -fsS "${BASE_URL}/healthz" >/dev/null; then
      return 0
    fi
    sleep 2
  done

  echo "gateway not healthy within 30s" >&2
  ${COMPOSE_CMD} logs >&2 || true
  return 1
}

teardown_file() {
  # --profile '*' so profile-gated services (e.g. gateway-tls) are torn down too.
  ${COMPOSE_CMD} --profile '*' down -v -t 0 >/dev/null || true
}

@test "/healthz is public" {
  run curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/healthz"
  [[ "${output}" == "200" ]]
}

@test "/styles.css is public" {
  run curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/styles.css"
  [[ "${output}" == "200" ]]
}

@test "unauthenticated /ide/ redirects to /login" {
  run curl -s -o /dev/null -w '%{http_code} %{redirect_url}' "${BASE_URL}/ide/"
  [[ "${output}" == 302* ]]
  [[ "${output}" == *"/login"* ]]
}

@test "wrong password returns 401 and sets no cookie" {
  local jar="${BATS_TEST_TMPDIR}/wrong.jar"
  run curl -s -o /dev/null -w '%{http_code}' -c "${jar}" \
    --data 'password=definitely-wrong' "${BASE_URL}/login"
  [[ "${output}" == "401" ]]
  run grep -c gw_auth "${jar}"
  [[ "${output}" == "0" ]]
}

@test "correct password sets the gw_auth cookie" {
  local jar="${BATS_TEST_TMPDIR}/ok.jar"
  curl -s -o /dev/null -c "${jar}" --data "password=${PASSWORD}" "${BASE_URL}/login"
  run grep -c gw_auth "${jar}"
  [[ "${output}" -ge 1 ]]
}

@test "authenticated / serves the configured landing page" {
  local jar="${BATS_TEST_TMPDIR}/landing.jar"
  curl -s -o /dev/null -c "${jar}" --data "password=${PASSWORD}" "${BASE_URL}/login"

  run curl -s -b "${jar}" "${BASE_URL}/"
  [[ "${output}" == *"interview-labs workspace"* ]]
}

@test "valid cookie proxies /ide — page and static asset" {
  local jar="${BATS_TEST_TMPDIR}/ide.jar"
  curl -s -o /dev/null -c "${jar}" --data "password=${PASSWORD}" "${BASE_URL}/login"
  run curl -s -b "${jar}" "${BASE_URL}/ide/foo"
  [[ "${output}" == *"ide-sim uri=/foo"* ]]

  # the sim's own stylesheet is served as a real file (text/css) through the proxy
  run curl -s -D - -o /dev/null -b "${jar}" "${BASE_URL}/ide/sim.css"
  [[ "${output}" == *"200"* ]]
  [[ "${output}" == *"text/css"* ]]
}

@test "valid cookie proxies /term with the prefix stripped" {
  local jar="${BATS_TEST_TMPDIR}/term.jar"
  curl -s -o /dev/null -c "${jar}" --data "password=${PASSWORD}" "${BASE_URL}/login"
  run curl -s -b "${jar}" "${BASE_URL}/term/bar"
  [[ "${output}" == *"term-sim uri=/bar"* ]]
}

@test "forged cookie is rejected" {
  run curl -s -o /dev/null -w '%{http_code}' \
    --cookie 'gw_auth=9999999999.deadbeef' "${BASE_URL}/ide/"
  [[ "${output}" == "302" ]]
}

@test "login rate limit returns 429 after the burst" {
  local last i
  for (( i = 0; i < 7; i++ )); do
    last=$(curl -s -o /dev/null -w '%{http_code}' \
      --data 'password=definitely-wrong' "${BASE_URL}/login")
  done
  [[ "${last}" == "429" ]]
}

@test "logs contain an audit line but not the password" {
  curl -s -o /dev/null --data "password=${PASSWORD}" "${BASE_URL}/login"
  run ${COMPOSE_CMD} logs gateway
  [[ "${output}" == *"login success"* ]]
  [[ "${output}" != *"${PASSWORD}"* ]]
}

@test "gateway serves TLS and shows login over https" {
  "${BATS_TEST_DIRNAME}/tls/gen-cert.sh"
  ${COMPOSE_CMD} up -d --build gateway-tls >/dev/null

  local i
  for (( i = 0; i < 15; i++ )); do
    if curl -kfsS "https://127.0.0.1:8443/healthz" >/dev/null; then
      break
    fi
    sleep 2
  done

  run curl -ks "https://127.0.0.1:8443/healthz"
  [[ "${status}" -eq 0 ]]

  run curl -ks "https://127.0.0.1:8443/login"
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"Sign in"* ]]
}
