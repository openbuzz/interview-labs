#!/usr/bin/env bats

load 'helpers'

@test "init shows the provider menu with todo badges" {
  tmux_start "init-menu" "$(tui_cmd 'interview init')"
  tmux_wait_for "Select a provider to configure:"

  tmux_snapshot "init-menu"
}

@test "ESC leaves init with the setup summary" {
  tmux_start "init-exit" "$(tui_cmd 'interview init')"
  tmux_wait_for "Select a provider to configure:"

  tmux_send Escape
  tmux_wait_done

  tmux_snapshot "init-exit"
  [[ "$(tmux_exit_code)" == "0" ]]
}
