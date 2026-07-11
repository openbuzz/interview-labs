# shellcheck shell=bash

# Tmux harness for TUI golden-frame tests (runs inside the docker image).
#
# Each test spawns a headless tmux session with a fixed 120x80 viewport,
# drives the interview binary via send-keys, captures the rendered screen
# via capture-pane, and diffs it against a golden in tests/tui/snapshots/.
# UPDATE_GOLDENS=1 rewrites goldens instead of diffing.

TUI_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly TUI_DIR
readonly SNAPSHOT_DIR="${TUI_DIR}/snapshots"
readonly TMUX_DONE_MARKER="___TUI_TEST_DONE___"

TMUX_SESSION=""

# Drop everything from the done marker to EOF: the wrapper appends the marker
# plus sleep padding after the real output so the pane survives for capture.
_strip_after_done_marker() {
  sed "/${TMUX_DONE_MARKER}/,\$d"
}

# Peel trailing blank lines off the captured 80-row pane.
_strip_trailing_blank_lines() {
  sed -e :a -e '/^$/{$d;N;ba' -e '}'
}

# Start a fresh headless session running cmd; the wrapper records the exit
# code next to the done marker and keeps the pane alive for capture.
tmux_start() {
  local name="$1"
  local cmd="$2"
  TMUX_SESSION="tui-${name}-$$"

  tmux kill-session -t "${TMUX_SESSION}" 2>/dev/null || true
  tmux new-session -d -s "${TMUX_SESSION}" -x 120 -y 80 \
    "${cmd}; printf '\n%s %s\n' '${TMUX_DONE_MARKER}' \"\$?\"; sleep 60"
}

# Poll the pane for a literal pattern; ~10s budget.
tmux_wait_for() {
  local pattern="$1"
  local _
  for _ in $(seq 1 100); do
    tmux capture-pane -pt "${TMUX_SESSION}" | grep -qF -- "${pattern}" && return 0
    sleep 0.1
  done

  echo "timeout waiting for: ${pattern}" >&2
  tmux capture-pane -pt "${TMUX_SESSION}" >&2
  return 1
}

tmux_send() {
  tmux send-keys -t "${TMUX_SESSION}" "$@"
}

tmux_wait_done() {
  tmux_wait_for "${TMUX_DONE_MARKER}"
}

# Exit code the wrapper recorded next to the marker.
tmux_exit_code() {
  tmux capture-pane -pt "${TMUX_SESSION}" \
    | grep -F -- "${TMUX_DONE_MARKER}" | awk '{print $2}'
}

# Diff the current frame against its golden (UPDATE_GOLDENS=1 rewrites it).
tmux_snapshot() {
  local name="$1"
  local golden="${SNAPSHOT_DIR}/${name}.txt"
  local frame
  frame="$(tmux capture-pane -pt "${TMUX_SESSION}" \
    | _strip_after_done_marker | _strip_trailing_blank_lines)"

  if [[ "${UPDATE_GOLDENS:-}" == "1" ]]; then
    mkdir -p "${SNAPSHOT_DIR}"
    printf '%s\n' "${frame}" >"${golden}"
    return 0
  fi

  diff -u "${golden}" <(printf '%s\n' "${frame}")
}

tmux_kill() {
  [[ -n "${TMUX_SESSION}" ]] && tmux kill-session -t "${TMUX_SESSION}" 2>/dev/null
  TMUX_SESSION=""
  return 0
}
