#!/usr/bin/env bash

set -e -u -o pipefail

code-server \
  --auth none \
  --bind-addr 0.0.0.0:8080 \
  --disable-telemetry --disable-update-check \
  /home/user/workspace &
cs_pid=$!

# TTYD_BASE_PATH (the gateway sets /term) hosts ttyd under a sub-path; empty serves at root.
ttyd_args=(--port 7681 --interface 0.0.0.0 --writable)
[[ -n "${TTYD_BASE_PATH:-}" ]] && ttyd_args+=(--base-path "${TTYD_BASE_PATH}")
ttyd "${ttyd_args[@]}" bash &

# Container lifetime follows code-server; if ttyd (the fallback terminal) dies, the IDE stays up.
wait "${cs_pid}"
