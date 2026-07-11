#!/usr/bin/env bats

load 'helpers'

@test "slug-less destroy offers the session picker; ESC aborts" {
  seed_session "brave-fox-a1b2" "203.0.113.7" "ready" 30
  seed_session "calm-owl-c3d4" "203.0.113.8" "ready" 10

  tmux_start "destroy-picker" "$(tui_cmd 'interview destroy')"
  tmux_wait_for "Session"

  tmux_snapshot "destroy-picker"

  tmux_send Escape
  tmux_wait_done
  [[ "$(tmux_exit_code)" != "0" ]]
}
