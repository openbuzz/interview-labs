#!/usr/bin/env bats

load 'helpers'

@test "launch offers the bundle picker first" {
  tmux_start "bundle-picker" "$(tui_cmd 'interview launch')"
  tmux_wait_for "Select an interview bundle"

  tmux_snapshot "bundle-picker"

  tmux_send Escape
  tmux_wait_done
  [[ "$(tmux_exit_code)" != "0" ]]
}
