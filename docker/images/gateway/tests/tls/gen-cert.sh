#!/usr/bin/env bash

# Generate a throwaway self-signed cert for the TLS smoke test.
# TEST USE ONLY — never a production certificate.

set -e -u -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

[[ -f "${SCRIPT_DIR}/cert.pem" && -f "${SCRIPT_DIR}/key.pem" ]] && exit 0

openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
  -keyout "${SCRIPT_DIR}/key.pem" -out "${SCRIPT_DIR}/cert.pem" \
  -days 3650 -nodes -subj "/CN=127.0.0.1" \
  -addext "subjectAltName=IP:127.0.0.1"

# Widen key permissions so the non-root gateway container can read it.
chmod 644 "${SCRIPT_DIR}/key.pem"
