#!/usr/bin/env bats

load 'helpers'

@test "bare interview shows the menu" {
  tmux_start "root-menu" "$(tui_cmd interview)"
  tmux_wait_for "What do you want to do?"

  tmux_snapshot "root-menu"
}

@test "menu navigates and ESC exits cleanly" {
  tmux_start "root-menu-nav" "$(tui_cmd interview)"
  tmux_wait_for "What do you want to do?"

  tmux_send Down Down
  sleep 0.3
  tmux_snapshot "root-menu-nav"

  tmux_send Escape
  tmux_wait_done
  [[ "$(tmux_exit_code)" == "0" ]]
}
