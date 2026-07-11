#!/usr/bin/env bats

load 'helpers'

@test "harness scripts pass shellcheck" {
  shellcheck tests/tui/lib/tmux.sh tests/tui/helpers.bash
}

@test "wrapper records exit codes" {
  tmux_start "harness-exit" "false"
  tmux_wait_done

  [[ "$(tmux_exit_code)" == "1" ]]
}
